package archiver

import (
	"archive/zip"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/itchio/wharf/counter"
	"github.com/itchio/wharf/pwr"
	"github.com/itchio/wharf/sync"
	"github.com/itchio/wharf/tlc"
)

func ExtractZip(archive string, dir string, consumer *pwr.StateConsumer) (*ExtractResult, error) {
	consumer.Infof("Extracting %s to %s", archive, dir)

	dirCount := 0
	regCount := 0
	symlinkCount := 0

	reader, err := zip.OpenReader(archive)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	for _, file := range reader.File {
		err = func() error {
			rel := file.Name
			filename := path.Join(dir, filepath.FromSlash(rel))

			info := file.FileInfo()
			mode := info.Mode()

			fileReader, err := file.Open()
			if err != nil {
				return err
			}
			defer fileReader.Close()

			if info.IsDir() {
				err = Mkdir(filename)
				if err != nil {
					return err
				}
				dirCount++
			} else if mode&os.ModeSymlink > 0 {
				linkname, err := ioutil.ReadAll(fileReader)
				err = Symlink(string(linkname), filename, consumer)
				if err != nil {
					return err
				}
				symlinkCount++
			} else {
				err = CopyFile(filename, os.FileMode(mode&LuckyMode|ModeMask), fileReader, consumer)
				if err != nil {
					return err
				}
				regCount++
			}

			return nil
		}()
		if err != nil {
			return nil, err
		}
	}

	return &ExtractResult{
		Dirs:     dirCount,
		Files:    regCount,
		Symlinks: symlinkCount,
	}, nil
}

func CompressZip(archive string, container *tlc.Container, pool sync.FilePool, consumer *pwr.StateConsumer) (*CompressResult, error) {
	var uncompressedSize int64
	var compressedSize int64

	archiveWriter, err := os.Create(archive)
	if err != nil {
		return nil, err
	}

	archiveCounter := counter.NewWriter(archiveWriter)

	zipWriter := zip.NewWriter(archiveCounter)
	defer zipWriter.Close()

	for _, dir := range container.Dirs {
		fh := zip.FileHeader{
			Name: dir.Path + "/",
		}
		fh.SetMode(os.FileMode(dir.Mode))

		_, err := zipWriter.CreateHeader(&fh)
		if err != nil {
			return nil, err
		}
	}

	for fileIndex, file := range container.Files {
		fh := zip.FileHeader{
			Name:               file.Path,
			UncompressedSize64: uint64(file.Size),
			Method:             zip.Deflate,
		}
		fh.SetMode(os.FileMode(file.Mode))

		entryWriter, err := zipWriter.CreateHeader(&fh)
		if err != nil {
			return nil, err
		}

		entryReader, err := pool.GetReader(int64(fileIndex))
		if err != nil {
			return nil, err
		}

		copiedBytes, err := io.Copy(entryWriter, entryReader)
		if err != nil {
			return nil, err
		}

		uncompressedSize += copiedBytes
	}

	for _, symlink := range container.Symlinks {
		fh := zip.FileHeader{
			Name: symlink.Path,
		}
		fh.SetMode(os.FileMode(symlink.Mode))

		entryWriter, err := zipWriter.CreateHeader(&fh)
		if err != nil {
			return nil, err
		}

		entryWriter.Write([]byte(symlink.Dest))
	}

	err = zipWriter.Flush()
	if err != nil {
		return nil, err
	}
	compressedSize = archiveCounter.Count()

	return &CompressResult{
		UncompressedSize: uncompressedSize,
		CompressedSize:   compressedSize,
	}, nil
}
