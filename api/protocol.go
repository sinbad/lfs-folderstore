package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sinbad/lfs-folderstore/util"
)

// Header struct
type Header struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Action struct
type Action struct {
	Href      string            `json:"href"`
	Header    map[string]string `json:"header,omitempty"`
	ExpiresAt time.Time         `json:"expires_at,omitempty"`
}

// TransferError struct
type TransferError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Request struct which can accept anything
type Request struct {
	Event               string  `json:"event"`
	Operation           string  `json:"operation"`
	Concurrent          bool    `json:"concurrent"`
	ConcurrentTransfers int     `json:"concurrenttransfers"`
	Oid                 string  `json:"oid"`
	Size                int64   `json:"size"`
	Path                string  `json:"path"`
	Action              *Action `json:"action"`
}

// InitResponse with response for init
type InitResponse struct {
	Error *TransferError `json:"error,omitempty"`
}

// TransferResponse generic transfer response
type TransferResponse struct {
	Event string         `json:"event"`
	Oid   string         `json:"oid"`
	Path  string         `json:"path,omitempty"` // always blank for upload
	Error *TransferError `json:"error,omitempty"`
}

// ProgressResponse blah
type ProgressResponse struct {
	Event          string `json:"event"`
	Oid            string `json:"oid"`
	BytesSoFar     int64  `json:"bytesSoFar"`
	BytesSinceLast int    `json:"bytesSinceLast"`
}

// SendResponse sends an actual response to lfs
func SendResponse(r interface{}, writer, errWriter *bufio.Writer) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	// Line oriented JSON
	b = append(b, '\n')
	_, err = writer.Write(b)
	if err != nil {
		return err
	}
	writer.Flush()
	util.WriteToStderr(fmt.Sprintf("Sent message %v", string(b)), errWriter)
	return nil
}

// SendTransferError sends an error back to lfs
func SendTransferError(oid string, code int, message string, writer, errWriter *bufio.Writer) {
	resp := &TransferResponse{"complete", oid, "", &TransferError{code, message}}
	err := SendResponse(resp, writer, errWriter)
	if err != nil {
		util.WriteToStderr(fmt.Sprintf("Unable to send transfer error: %v\n", err), errWriter)
	}
}

// SendProgress reports progress on operations
func SendProgress(oid string, bytesSoFar int64, bytesSinceLast int, writer, errWriter *bufio.Writer) {
	resp := &ProgressResponse{"progress", oid, bytesSoFar, bytesSinceLast}
	err := SendResponse(resp, writer, errWriter)
	if err != nil {
		util.WriteToStderr(fmt.Sprintf("Unable to send progress update: %v\n", err), errWriter)
	}
}
