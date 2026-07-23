package idxfile_test

import (
	"bytes"
	"io"

	"github.com/go-git/go-git/v5/plumbing"
	. "github.com/go-git/go-git/v5/plumbing/format/idxfile"

	fixtures "github.com/go-git/go-git-fixtures/v4"
	. "gopkg.in/check.v1"
)

func (s *IdxfileSuite) TestDecodeEncode(c *C) {
	fixtures.ByTag("packfile").Test(c, func(f *fixtures.Fixture) {
		expected, err := io.ReadAll(f.Idx())
		c.Assert(err, IsNil)

		idx := new(MemoryIndex)
		d := NewDecoder(bytes.NewBuffer(expected))
		err = d.Decode(idx)
		c.Assert(err, IsNil)

		result := bytes.NewBuffer(nil)
		e := NewEncoder(result)
		size, err := e.Encode(idx, nil)
		c.Assert(err, IsNil)

		c.Assert(size, Equals, len(expected))
		c.Assert(result.Bytes(), DeepEquals, expected)
	})
}

func (s *IdxfileSuite) TestEncodeStatusChanCompletes(c *C) {
	f := fixtures.ByTag("packfile").One()
	idx := new(MemoryIndex)
	err := NewDecoder(f.Idx()).Decode(idx)
	c.Assert(err, IsNil)
	count, err := idx.Count()
	c.Assert(err, IsNil)
	occupiedBuckets := 0
	var previous uint32
	for _, cumulative := range idx.Fanout {
		if cumulative != previous {
			occupiedBuckets++
			previous = cumulative
		}
	}
	c.Assert(int(count) > occupiedBuckets, Equals, true)

	updates := make(chan plumbing.StatusUpdate, 1024)
	_, err = NewEncoder(io.Discard).Encode(idx, updates)
	c.Assert(err, IsNil)
	close(updates)

	last := make(map[plumbing.StatusStage]plumbing.StatusUpdate)
	for update := range updates {
		last[update.Stage] = update
	}
	for _, stage := range []plumbing.StatusStage{
		plumbing.StatusIndexHash,
		plumbing.StatusIndexCRC,
		plumbing.StatusIndexOffset,
	} {
		update, ok := last[stage]
		c.Assert(ok, Equals, true)
		c.Assert(update.ObjectsDone, Equals, update.ObjectsTotal)
	}
}
