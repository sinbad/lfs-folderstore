package service

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/sinbad/lfs-folderstore/api"
	"github.com/sinbad/lfs-folderstore/util"
)

// Serve starts the protocol server
func Serve(baseDir string, stdin io.Reader, stdout, stderr io.Writer) {

	scanner := bufio.NewScanner(stdin)
	writer := bufio.NewWriter(stdout)
	errWriter := bufio.NewWriter(stderr)

	for scanner.Scan() {
		line := scanner.Text()
		var req api.Request

		if err := json.Unmarshal([]byte(line), &req); err != nil {
			util.WriteToStderr(fmt.Sprintf("Unable to parse request: %v\n", line), errWriter)
			continue
		}

		switch req.Event {
		case "init":
			resp := &api.InitResponse{}
			if len(baseDir) == 0 {
				resp.Error = &api.TransferError{Code: 9, Message: "Base directory not specified, check config"}
			} else {
				util.WriteToStderr(fmt.Sprintf("Initialised lfs-folderstore custom adapter for %s\n", req.Operation), errWriter)
			}
			api.SendResponse(resp, writer, errWriter)
		case "download":
			util.WriteToStderr(fmt.Sprintf("Received download request for %s\n", req.Oid), errWriter)
			retrieve(baseDir, req.Oid, req.Size, req.Action, writer, errWriter)
		case "upload":
			util.WriteToStderr(fmt.Sprintf("Received upload request for %s\n", req.Oid), errWriter)
			store(baseDir, req.Oid, req.Size, req.Action, req.Path, writer, errWriter)
		case "terminate":
			util.WriteToStderr("Terminating test custom adapter gracefully.\n", errWriter)
			break
		}
	}

}

func storagePath(baseDir string, oid string) string {
	// Use same folder split as lfs itself
	fld := filepath.Join(baseDir, oid[0:2], oid[2:4])
	os.MkdirAll(fld, os.ModePerm)
	return filepath.Join(fld, oid)
}

func retrieve(baseDir string, oid string, size int64, a *api.Action, writer, errWriter *bufio.Writer) {

	// We just use a shared DB of objects stored by OID across all repos
	// If user wants to separate, can just use a different folder
	filePath := storagePath(baseDir, oid)
	stat, err := os.Stat(filePath)
	if err != nil {
		api.SendTransferError(oid, 3, fmt.Sprintf("Cannot stat %q: %v", filePath, err), writer, errWriter)
		return
	}

	if !stat.Mode().IsRegular() {
		api.SendTransferError(oid, 4, fmt.Sprintf("Store corruption, %q is not a regular file", filePath), writer, errWriter)
		return
	}

	// Copy to temp, since LFS will rename this to final location
	dlFile, err := ioutil.TempFile("", "lfsfolderstore")
	if err != nil {
		api.SendTransferError(oid, 5, fmt.Sprintf("Error creating temp file for %q: %v", filePath, err), writer, errWriter)
		return
	}
	defer dlFile.Close()
	dlfilename := dlFile.Name()

	f, err := os.OpenFile(filePath, os.O_RDONLY, 0644)
	if err != nil {
		api.SendTransferError(oid, 6, fmt.Sprintf("Cannot read data from %q: %v", filePath, err), writer, errWriter)
		os.Remove(dlfilename)
		return
	}
	defer f.Close()

	cb := func(totalSize, readSoFar int64, readSinceLast int) error {
		api.SendProgress(oid, readSoFar, readSinceLast, writer, errWriter)
		return nil
	}

	err = copyFileContents(stat.Size(), f, dlFile, cb)
	if err != nil {
		api.SendTransferError(oid, 7, fmt.Sprintf("Error copy file from %q: %v", filePath, err), writer, errWriter)
		dlFile.Close()
		os.Remove(dlfilename)
		return
	}

	if err := dlFile.Close(); err != nil {
		api.SendTransferError(oid, 5, fmt.Sprintf("can't close tempfile %q: %v", dlfilename, err), writer, errWriter)
		os.Remove(dlfilename)
		return
	}

	// completed
	complete := &api.TransferResponse{Event: "complete", Oid: oid, Path: dlfilename, Error: nil}
	err = api.SendResponse(complete, writer, errWriter)
	if err != nil {
		util.WriteToStderr(fmt.Sprintf("Unable to send completion message: %v\n", err), errWriter)
	}
}

type copyCallback func(totalSize int64, readSoFar int64, readSinceLast int) error

func copyFileContents(size int64, src, dst *os.File, cb copyCallback) error {
	// copy file in chunks (4K is usual block size of disks)
	const blockSize int64 = 4 * 1024 * 16

	// Read precisely the correct number of bytes
	bytesLeft := size
	for bytesLeft > 0 {
		nextBlock := blockSize
		if nextBlock < bytesLeft {
			nextBlock = bytesLeft
		}
		n, err := io.CopyN(dst, src, nextBlock)
		bytesLeft -= n
		if err != nil && err != io.EOF {
			return err
		}
		readSoFar := size - bytesLeft
		if cb != nil {
			cb(size, readSoFar, int(n))
		}
	}
	return nil
}

func store(baseDir string, oid string, size int64, a *api.Action, fromPath string, writer, errWriter *bufio.Writer) {
	statFrom, err := os.Stat(fromPath)
	if err != nil {
		api.SendTransferError(oid, 13, fmt.Sprintf("Cannot stat %q: %v", fromPath, err), writer, errWriter)
		return
	}

	destPath := storagePath(baseDir, oid)

	statDest, err := os.Stat(destPath)
	if err == nil {
		// if file exists, skip if already the same size
		if statFrom.Size() == statDest.Size() {
			util.WriteToStderr(fmt.Sprintf("Skipping %v, already stored", oid), errWriter)

			// send full progress
			api.SendProgress(oid, statFrom.Size(), int(statFrom.Size()), writer, errWriter)
			// send completion
			complete := &api.TransferResponse{Event: "complete", Oid: oid, Error: nil}
			err = api.SendResponse(complete, writer, errWriter)
			if err != nil {
				util.WriteToStderr(fmt.Sprintf("Unable to send completion message: %v\n", err), errWriter)
			}
			return
		}
	}

	err = os.MkdirAll(filepath.Dir(destPath), 0755)
	if err != nil {
		api.SendTransferError(oid, 14, fmt.Sprintf("Cannot create dir %q: %v", filepath.Dir(destPath), err), writer, errWriter)
		return
	}

	// write a temp file in same folder, then rename
	tempPath := fmt.Sprintf("%v.tmp", destPath)
	if _, err := os.Stat(tempPath); err == nil {
		// delete temp file
		err := os.Remove(tempPath)
		if err != nil && !os.IsNotExist(err) {
			api.SendTransferError(oid, 14, fmt.Sprintf("Cannot remove existing temp file %q: %v", tempPath, err), writer, errWriter)
			return
		}
	}

	srcf, err := os.OpenFile(fromPath, os.O_RDONLY, 0644)
	if err != nil {
		api.SendTransferError(oid, 15, fmt.Sprintf("Cannot read data from %q: %v", fromPath, err), writer, errWriter)
		return
	}
	defer srcf.Close()

	dstf, err := os.OpenFile(tempPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, statFrom.Mode())
	if err != nil {
		api.SendTransferError(oid, 16, fmt.Sprintf("Cannot open temp file for writing %q: %v", tempPath, err), writer, errWriter)
		return
	}
	defer dstf.Close()

	cb := func(totalSize, readSoFar int64, readSinceLast int) error {
		api.SendProgress(oid, readSoFar, readSinceLast, writer, errWriter)
		return nil
	}

	err = copyFileContents(statFrom.Size(), srcf, dstf, cb)
	if err != nil {
		api.SendTransferError(oid, 17, fmt.Sprintf("Error writing temp file %q: %v", tempPath, err), writer, errWriter)
		dstf.Close()
		os.Remove(tempPath)
		return
	}

	// now rename
	dstf.Close()
	err = os.Rename(tempPath, destPath)
	if err != nil {
		api.SendTransferError(oid, 18, fmt.Sprintf("Error moving temp file to final location: %v", err), writer, errWriter)
		os.Remove(tempPath)
		return
	}

	// completed
	complete := &api.TransferResponse{Event: "complete", Oid: oid, Error: nil}
	err = api.SendResponse(complete, writer, errWriter)
	if err != nil {
		util.WriteToStderr(fmt.Sprintf("Unable to send completion message: %v\n", err), errWriter)
	}

}
