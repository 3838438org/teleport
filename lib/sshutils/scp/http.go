/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package scp

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gravitational/teleport/lib/httplib"

	"github.com/gravitational/trace"
)

type HTTPTransferCommand struct {
	Command
	Writer         http.ResponseWriter
	Reader         io.ReadCloser
	UploadFileName string
}

func CreateHTTPUploadCommand(remoteLocation string, reader io.ReadCloser, progress io.Writer) (Command, error) {
	_, filename := filepath.Split(remoteLocation)
	if filename == "" {
		return nil, trace.BadParameter("invalid file path: %v", filename)
	}

	webCmd := HTTPTransferCommand{
		Reader:         reader,
		UploadFileName: filename,
	}

	cfg := Parameters{}
	cfg.ProgressWriter = progress
	cfg.RemoteFileLocation = remoteLocation
	cfg.FileSystem = &WebFS{
		cmd: webCmd,
	}

	cfg.Flags.Target = []string{filename}

	cmd, err := CreateUploadCommand(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	webCmd.Command = cmd

	return webCmd, nil
}

func CreateHTTPDownloadCommand(remoteLocation string, w http.ResponseWriter, progress io.Writer) (Command, error) {
	_, filename := filepath.Split(remoteLocation)
	if filename == "" {
		return nil, trace.BadParameter("invalid file path: %v", filename)
	}

	webCmd := HTTPTransferCommand{Writer: w}

	cfg := Parameters{}
	cfg.ProgressWriter = progress
	cfg.Flags.Target = []string{filename}
	cfg.RemoteFileLocation = remoteLocation
	cfg.FileSystem = &WebFS{
		cmd: webCmd,
	}

	cmd, err := CreateDownloadCommand(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	webCmd.Command = cmd

	return webCmd, nil
}

type WebFS struct {
	cmd HTTPTransferCommand
}

func (l *WebFS) SetChmod(path string, mode int) error {
	return nil
}

func (l *WebFS) MkDir(path string, mode int) error {
	return trace.BadParameter("copying directories is not supported in http file transfer")
}

func (l *WebFS) IsDir(path string) bool {
	return false
}

func (l *WebFS) OpenFile(filePath string) (io.ReadCloser, error) {
	if l.cmd.Reader == nil {
		return nil, trace.BadParameter("missing Reader")
	}

	return l.cmd.Reader, trace.BadParameter("not implemented")
}

func (l *WebFS) CreateFile(filePath string, length uint64) (io.WriteCloser, error) {
	_, filename := filepath.Split(filePath)
	contentLength := strconv.FormatUint(length, 10)
	header := l.cmd.Writer.Header()

	httplib.SetNoCacheHeaders(header)
	httplib.SetNoSniff(header)
	header.Set("Content-Length", contentLength)
	header.Set("Content-Type", "application/octet-stream")
	header.Set("Content-Disposition", fmt.Sprintf(`attachment;filename="%v"`, filename))
	return &WebReadWrite{
		Writer: l.cmd.Writer,
	}, nil
}

func (l *WebFS) GetFileInfo(filePath string) (FileInfo, error) {
	return &httpFileInfo{
		fileName: l.cmd.UploadFileName,
	}, nil
}

type httpFileInfo struct {
	isRecursive bool
	filePath    string
	fileName    string
	FileInfo
}

func (l *httpFileInfo) IsDir() bool {
	return false
}

func (l *httpFileInfo) GetName() string {
	return l.fileName
}

func (l *httpFileInfo) GetPath() string {
	return l.filePath
}

func (l *httpFileInfo) GetSize() int64 {
	return l.fileInfo.Size()
}

func (l *httpFileInfo) ReadDir() ([]FileInfo, error) {
	return nil, trace.BadParameter("not implemented")
}

func (l *httpFileInfo) GetModePerm() os.FileMode {
	// Only the owner can read and write, but not execute the file.
	// Everyone else can read and execute, but cannot modify the file.
	return 0655
}

type WebReadWrite struct {
	io.Writer
	io.Reader
}

//func (wr *WebReadWrite) Read(b []byte) (int, error) {
//	return wr.Read(b)
//}

//func (wr *WebReadWrite) Write(b []byte) (int, error) {
//	return wr.Write(b)
//}

func (wr *WebReadWrite) Close() error {
	return nil
}
