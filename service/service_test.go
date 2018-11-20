package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/sinbad/lfs-folderstore/api"
	"github.com/stretchr/testify/assert"
)

func TestStoragePath(t *testing.T) {
	type args struct {
		baseDir string
		oid     string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// platform-specific tests still run but use filepath.Join to make consistent
		{
			name: "Windows drive",
			args: args{baseDir: `C:/Storage/Dir`, oid: "123456789abcdef"},
			want: filepath.Join(`C:/Storage/Dir`, "12", "34", "123456789abcdef"),
		},
		{
			name: "Windows drive with space",
			args: args{baseDir: `C:/Storage Path/Dir`, oid: "123456789abcdef"},
			want: filepath.Join(`C:/Storage Path/Dir`, "12", "34", "123456789abcdef"),
		},
		{
			name: "Windows share",
			args: args{baseDir: `\\MyServer\Storage Path\Dir`, oid: "123456789abcdef"},
			want: filepath.Join(`\\MyServer\Storage Path\Dir`, "12", "34", "123456789abcdef"),
		},
		{
			name: "Windows trailing separator",
			args: args{baseDir: `C:/Storage/Dir/`, oid: "123456789abcdef"},
			want: filepath.Join(`C:/Storage/Dir`, "12", "34", "123456789abcdef"),
		},
		{
			name: "Unix path",
			args: args{baseDir: `/home/bob/`, oid: "123456789abcdef"},
			want: filepath.Join(`/home/bob`, "12", "34", "123456789abcdef"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := storagePath(tt.args.baseDir, tt.args.oid); got != tt.want {
				t.Errorf("storagePath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func addUpload(t *testing.T, buf *bytes.Buffer, path, oid string, size int64) {
	req := &api.Request{
		Event:  "upload",
		Oid:    oid,
		Size:   size,
		Path:   path,
		Action: &api.Action{},
	}
	b, err := json.Marshal(req)
	assert.Nil(t, err)
	b = append(b, '\n')

	buf.Write(b)
}

func initUpload(buf *bytes.Buffer) {
	buf.WriteString(`{ "event": "init", "operation": "upload", "remote": "origin", "concurrent": true, "concurrenttransfers": 3 }`)
	buf.WriteString("\n")
}

func finishUpload(buf *bytes.Buffer) {
	buf.WriteString(`{ "event": "terminate" }`)
	buf.WriteString("\n")
}

func TestUpload(t *testing.T) {

	setup := setupUploadTest(t)
	defer os.RemoveAll(setup.localpath)
	defer os.RemoveAll(setup.remotepath)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Perform entire sequence
	Serve(setup.remotepath, bytes.NewReader(setup.inputBuffer.Bytes()), &stdout, &stderr)

	// Check reported progress and completion
	stdoutStr := stdout.String()
	// init report
	assert.Contains(t, stdoutStr, "{}")
	// progress & completion for each file (only 2 uploaded)
	for _, file := range setup.files {
		assert.Contains(t, stdoutStr, `{"event":"progress","oid":"`+file.oid)
		assert.Contains(t, stdoutStr, `{"event":"complete","oid":"`+file.oid)
	}

	// Check actual files are there
	for _, file := range setup.files {
		expectedPath := filepath.Join(setup.remotepath, file.oid[0:2], file.oid[2:4], file.oid)
		assert.FileExistsf(t, expectedPath, "Store file must exist: %v", expectedPath)

		// Check size of file
		s, _ := os.Stat(expectedPath)
		assert.Equal(t, file.size, s.Size())

		// Re-calculate hash to verify
		oid := calculateFileHash(t, expectedPath)
		assert.Equal(t, file.oid, oid)
	}

	// Now try to perform an upload with files 3 & 4 - only one is new
	setup2 := setupUploadTest2(t, setup.localpath, setup.remotepath)
	stdout.Reset()
	stderr.Reset()
	Serve(setup2.remotepath, bytes.NewReader(setup2.inputBuffer.Bytes()), &stdout, &stderr)

	stdoutStr = stdout.String()
	stderrStr := stderr.String()

	// First file should not be updated (is file3 in original)
	assert.Contains(t, stderrStr, "Skipping "+setup2.files[0].oid)

	// Make sure second file was uploaded
	for _, file := range setup2.files {
		assert.Contains(t, stdoutStr, `{"event":"progress","oid":"`+file.oid)
		assert.Contains(t, stdoutStr, `{"event":"complete","oid":"`+file.oid)
	}

	// Check actual files are there
	for _, file := range setup2.files {
		expectedPath := filepath.Join(setup.remotepath, file.oid[0:2], file.oid[2:4], file.oid)
		assert.FileExistsf(t, expectedPath, "Store file must exist: %v", expectedPath)

		// Check size of file
		s, _ := os.Stat(expectedPath)
		assert.Equal(t, file.size, s.Size())

		// Re-calculate hash to verify
		oid := calculateFileHash(t, expectedPath)
		assert.Equal(t, file.oid, oid)
	}

}

type testFile struct {
	path string
	size int64
	oid  string
}
type testSetup struct {
	localpath   string
	remotepath  string
	files       []testFile
	inputBuffer *bytes.Buffer
}

func setupUploadTest(t *testing.T) *testSetup {
	// Create 2 temporary dirs, pretending to be git repo and dest shared folder
	gitpath, err := ioutil.TempDir(os.TempDir(), "lfs-folderstore-test-local")
	assert.Nil(t, err, "Error creating temp git path")

	storepath, err := ioutil.TempDir(os.TempDir(), "lfs-folderstore-test-remote")
	assert.Nil(t, err, "Error creating temp shared path")

	testfiles := []testFile{
		{ // small file
			path: filepath.Join(gitpath, "file1"),
			size: 650,
		},
		{ // Multiple block file
			path: filepath.Join(gitpath, "file2"),
			size: 4 * 1024 * 16 * 2,
		},
		{ // Multiple block file with remainder
			path: filepath.Join(gitpath, "file3"),
			size: 4*1024*16*6 + 345,
		},
	}

	for i, file := range testfiles {
		// note must reindex since file is by value
		testfiles[i].oid = createTestFile(t, file.size, file.path)
	}

	// Construct an input buffer of commands to upload first 2 files
	var commandBuf bytes.Buffer
	initUpload(&commandBuf)

	for _, file := range testfiles {
		addUpload(t, &commandBuf, file.path, file.oid, file.size)
	}

	finishUpload(&commandBuf)

	return &testSetup{
		localpath:   gitpath,
		remotepath:  storepath,
		files:       testfiles,
		inputBuffer: &commandBuf,
	}

}

func setupUploadTest2(t *testing.T, gitpath, storepath string) *testSetup {

	testfiles := []testFile{
		{ // File 3 again
			path: filepath.Join(gitpath, "file3"),
			size: 4*1024*16*6 + 345,
		},
		{ // File 3 again
			path: filepath.Join(gitpath, "file4"),
			size: 4*1024*16*2 + 1020,
		},
	}

	testfiles[0].oid = calculateFileHash(t, testfiles[0].path)
	testfiles[1].oid = createTestFile(t, testfiles[1].size, testfiles[1].path)

	// Construct an input buffer of commands to upload first 2 files
	var commandBuf bytes.Buffer
	initUpload(&commandBuf)

	for _, file := range testfiles {
		addUpload(t, &commandBuf, file.path, file.oid, file.size)
	}

	finishUpload(&commandBuf)

	return &testSetup{
		localpath:   gitpath,
		remotepath:  storepath,
		files:       testfiles,
		inputBuffer: &commandBuf,
	}

}

func TestDownload(t *testing.T) {
	setup := setupDownloadTest(t)
	defer os.RemoveAll(setup.localpath)
	defer os.RemoveAll(setup.remotepath)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Perform entire sequence
	Serve(setup.remotepath, bytes.NewReader(setup.inputBuffer.Bytes()), &stdout, &stderr)

	// Check reported progress and completion
	stdoutStr := stdout.String()
	// init report
	assert.Contains(t, stdoutStr, "{}")
	// progress & completion for each file (only 2 Downloaded)
	for _, file := range setup.files {
		assert.Contains(t, stdoutStr, `{"event":"progress","oid":"`+file.oid)
		assert.Contains(t, stdoutStr, `{"event":"complete","oid":"`+file.oid)
	}

	// Check actual files are in the path specified
	// NB: won't be in the local store, because git-lfs moves into that location
	for _, file := range setup.files {
		assert.FileExistsf(t, file.path, "Local file must exist: %v", file.path)

		// Check size of file
		s, _ := os.Stat(file.path)
		assert.Equal(t, file.size, s.Size())

		// Re-calculate hash to verify
		oid := calculateFileHash(t, file.path)
		assert.Equal(t, file.oid, oid)
	}

	// No need to test partial download since git-lfs eliminates those,
	// custom adapter has no way to know what's in the local repo

}

func addDownload(t *testing.T, buf *bytes.Buffer, oid string, size int64) {
	req := &api.Request{
		Event:  "download",
		Oid:    oid,
		Size:   size,
		Action: &api.Action{},
	}
	b, err := json.Marshal(req)
	assert.Nil(t, err)
	b = append(b, '\n')

	buf.Write(b)
}

func initDownload(buf *bytes.Buffer) {
	buf.WriteString(`{ "event": "init", "operation": "download", "remote": "origin", "concurrent": true, "concurrenttransfers": 3 }`)
	buf.WriteString("\n")
}

func finishDownload(buf *bytes.Buffer) {
	buf.WriteString(`{ "event": "terminate" }`)
	buf.WriteString("\n")
}

func setupDownloadTest(t *testing.T) *testSetup {
	// Create 2 temporary dirs, pretending to be git repo and dest shared folder
	gitpath, err := ioutil.TempDir(os.TempDir(), "lfs-folderstore-test-local")
	assert.Nil(t, err, "Error creating temp git path")

	storepath, err := ioutil.TempDir(os.TempDir(), "lfs-folderstore-test-remote")
	assert.Nil(t, err, "Error creating temp shared path")

	testfiles := []testFile{
		{ // small file
			path: filepath.Join(storepath, "file6"),
			size: 1023,
		},
		{ // Multiple block file
			path: filepath.Join(storepath, "file7"),
			size: 4 * 1024 * 16 * 10,
		},
		{ // Multiple block file with remainder
			path: filepath.Join(storepath, "file8"),
			size: 4*1024*16*12 + 456,
		},
	}

	for i, file := range testfiles {
		oid := createTestFile(t, file.size, file.path)
		// move these to final location
		finalLocation := filepath.Join(storepath, oid[0:2], oid[2:4], oid)
		assert.Nil(t, os.MkdirAll(filepath.Dir(finalLocation), 0644))
		assert.Nil(t, os.Rename(file.path, finalLocation))
		// Must re-index since file is byval
		testfiles[i].path = finalLocation
		testfiles[i].oid = oid
	}

	// Construct an input buffer of commands to upload first 2 files
	var commandBuf bytes.Buffer
	initDownload(&commandBuf)

	for _, file := range testfiles {
		addDownload(t, &commandBuf, file.oid, file.size)
	}

	finishDownload(&commandBuf)

	return &testSetup{
		localpath:   gitpath,
		remotepath:  storepath,
		files:       testfiles,
		inputBuffer: &commandBuf,
	}

}

func createTestFile(t *testing.T, size int64, path string) string {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
	assert.Nil(t, err)
	defer f.Close()

	byteSnippet := []byte{
		1, 2, 3, 5, 7, 11, 13, 17, 19, 23, 29,
		31, 37, 41, 43, 47, 53, 59, 61, 67, 71,
		73, 79, 83, 89, 97, 101, 103, 107, 109, 113,
		127, 131, 137, 139, 149, 151, 157, 163, 167, 173,
		179, 181, 191, 193, 197, 199, 211, 223, 227, 229,
		233, 239, 241, 251,
	}

	oidHash := sha256.New()

	bytesLeft := size
	byteSnippetLen := int64(len(byteSnippet))
	for bytesLeft > 0 {
		c := len(byteSnippet)
		if bytesLeft < byteSnippetLen {
			c = int(bytesLeft)
		}
		_, err = f.Write(byteSnippet[0:c])
		oidHash.Write(byteSnippet[0:c])
		assert.Nil(t, err)
		bytesLeft -= byteSnippetLen
	}

	return hex.EncodeToString(oidHash.Sum(nil))
}

func calculateFileHash(t *testing.T, filepath string) string {
	hasher := sha256.New()
	f, err := os.OpenFile(filepath, os.O_RDONLY, 0644)
	assert.Nil(t, err)
	defer f.Close()
	_, err = io.Copy(hasher, f)
	assert.Nil(t, err)

	return hex.EncodeToString(hasher.Sum(nil))
}
