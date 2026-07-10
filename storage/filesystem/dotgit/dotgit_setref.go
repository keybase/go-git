package dotgit

import (
	"fmt"
	"os"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/utils/ioutil"

	"github.com/go-git/go-billy/v5"
)

func (d *DotGit) setRef(fileName, content string, old *plumbing.Reference) (err error) {
	if billy.CapabilityCheck(d.fs, billy.ReadAndWriteCapability) {
		return d.setRefRwfs(fileName, content, old)
	}

	return d.setRefNorwfs(fileName, content, old)
}

func (d *DotGit) setRefRwfs(fileName, content string, old *plumbing.Reference) (err error) {
	// If we are not checking an old ref, just truncate the file.
	mode := os.O_RDWR | os.O_CREATE
	if old == nil {
		mode |= os.O_TRUNC
	}

	f, err := d.fs.OpenFile(fileName, mode, 0666)
	if err != nil {
		return err
	}

	// KBFS-3703: recover from a failed write by restoring the previous ref
	// content. Without recovery, a partially-written ref file can corrupt the
	// repository. The plain ioutil.CheckClose only propagates the close error;
	// it does not attempt to restore the original content when the write fails.
	defer func() {
		realErr := err
		ioutil.CheckClose(f, &err)
		if err == nil {
			return
		}
		// Write failed with an existing ref; KBFS atomic semantics mean the
		// file was not modified — leave the error standing without restoring.
		if old != nil && realErr != nil {
			return
		}
		// If old is nil, the file was created new — remove it.
		if old == nil {
			_ = d.fs.Remove(fileName)
			return
		}
		// Restore the original ref content.
		var oldContent string
		switch old.Type() {
		case plumbing.SymbolicReference:
			oldContent = fmt.Sprintf("ref: %s\n", old.Target())
		case plumbing.HashReference:
			oldContent = fmt.Sprintln(old.Hash().String())
		}
		f2, openErr := d.fs.OpenFile(fileName, os.O_RDWR|os.O_TRUNC, 0666)
		if openErr != nil {
			return
		}
		_, _ = f2.Write([]byte(oldContent))
		_ = f2.Close()
	}()

	// Lock is unlocked by the deferred Close above. This is because Unlock
	// does not imply a fsync and thus there would be a race between
	// Unlock+Close and other concurrent writers. Adding Sync to go-billy
	// could work, but this is better (and avoids superfluous syncs).
	err = f.Lock()
	if err != nil {
		return err
	}

	// this is a no-op to call even when old is nil.
	err = d.checkReferenceAndTruncate(f, old)
	if err != nil {
		return err
	}

	_, err = f.Write([]byte(content))
	return err
}

// There are some filesystems that don't support opening files in RDWD mode.
// In these filesystems the standard SetRef function can not be used as it
// reads the reference file to check that it's not modified before updating it.
//
// This version of the function writes the reference without extra checks
// making it compatible with these simple filesystems. This is usually not
// a problem as they should be accessed by only one process at a time.
func (d *DotGit) setRefNorwfs(fileName, content string, old *plumbing.Reference) error {
	_, err := d.fs.Stat(fileName)
	if err == nil && old != nil {
		fRead, err := d.fs.Open(fileName)
		if err != nil {
			return err
		}

		ref, err := d.readReferenceFrom(fRead, old.Name().String())
		fRead.Close()

		if err != nil {
			return err
		}

		if ref.Hash() != old.Hash() {
			return fmt.Errorf("reference has changed concurrently")
		}
	}

	f, err := d.fs.Create(fileName)
	if err != nil {
		return err
	}

	defer f.Close()

	_, err = f.Write([]byte(content))
	return err
}
