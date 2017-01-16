package bsdiff

import "github.com/jgallagher/gosaca"

// Partitioned suffix array
type PSA struct {
	p          int
	buf        []byte
	boundaries []int

	I    []int
	done chan bool
}

type BucketGroup struct {
	numSuffixes   int
	bucketNumbers []int
}

func NewPSA(p int, buf []byte) *PSA {
	boundaries := make([]int, p+1)
	boundary := 0
	partitionSize := len(buf) / p

	for i := 0; i < p; i++ {
		boundaries[i] = boundary
		boundary += partitionSize
	}
	boundaries[p] = len(buf)

	sortDone := make(chan bool)
	I := make([]int, len(buf))

	for i := 0; i < p; i++ {
		go func(i int) {
			st := boundaries[i]
			en := boundaries[i+1]
			ws := &gosaca.WorkSpace{}
			ws.ComputeSuffixArray(buf[st:en], I[st:en])
			sortDone <- true
		}(i)
	}

	for i := 0; i < p; i++ {
		<-sortDone
	}

	psa := &PSA{
		p:          p,
		buf:        buf,
		I:          I,
		boundaries: boundaries,
	}

	return psa
}

func (psa *PSA) search(nbuf []byte) (pos, n int) {
	var bpos, bn, i int

	for i = 0; i < psa.p; i++ {
		st := psa.boundaries[i]
		en := psa.boundaries[i+1]

		ppos, pn := search(psa.I[st:en], psa.buf[st:en], nbuf, 0, en-st)
		if pn > bn {
			bn = pn
			bpos = ppos + st
		}
	}

	return bpos, bn
}
