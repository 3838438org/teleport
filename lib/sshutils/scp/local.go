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

// Package scp handles file uploads and downloads via scp command
package scp

import (
	"io"
	"os"
	"path/filepath"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

type FileSystem interface {
	IsDir(path string) bool
	GetFileInfo(filePath string) (FileInfo, error)
	MkDir(path string, mode int) error
	OpenFile(filePath string) (io.ReadCloser, error)
	CreateFile(filePath string, length uint64) (io.WriteCloser, error)
	SetChmod(path string, mode int) error
}

type FileInfo interface {
	IsDir() bool
	ReadDir() ([]FileInfo, error)
	GetName() string
	GetPath() string
	GetModePerm() os.FileMode
	GetSize() int64
}

type LocalFS struct {
}

func (l *LocalFS) SetChmod(path string, mode int) error {
	chmode := os.FileMode(mode & int(os.ModePerm))
	if err := os.Chmod(path, chmode); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (l *LocalFS) MkDir(path string, mode int) error {
	fileMode := os.FileMode(mode & int(os.ModePerm))
	err := os.MkdirAll(path, fileMode)
	if err != nil && !os.IsExist(err) {
		return trace.Wrap(err)
	}

	return nil
}

func (l *LocalFS) IsDir(path string) bool {
	return utils.IsDir(path)
}

func (l *LocalFS) OpenFile(filePath string) (io.ReadCloser, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return f, nil
}

func (l *LocalFS) GetFileInfo(filePath string) (FileInfo, error) {
	info, err := makeFileInfo(filePath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return info, nil
}

func makeFileInfo(filePath string) (FileInfo, error) {
	f, err := os.Stat(filePath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &fileInfo{
		filePath: filePath,
		fileInfo: f}, nil
}

func (l *LocalFS) CreateFile(filePath string, length uint64) (io.WriteCloser, error) {
	f, err := os.Create(filePath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return f, nil
}

type fileInfo struct {
	isRecursive bool
	filePath    string
	fileInfo    os.FileInfo
	FileInfo
}

func (l *fileInfo) IsDir() bool {
	return l.fileInfo.IsDir()
}

func (l *fileInfo) GetName() string {
	return l.fileInfo.Name()
}

func (l *fileInfo) GetPath() string {
	return l.filePath
}

func (l *fileInfo) GetSize() int64 {
	return l.fileInfo.Size()
}

func (l *fileInfo) ReadDir() ([]FileInfo, error) {
	f, err := os.Open(l.filePath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	fis, err := f.Readdir(0)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	infos := make([]FileInfo, len(fis))
	for i := range fis {
		fi := fis[i]
		info, err := makeFileInfo(filepath.Join(l.GetPath(), fi.Name()))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		infos[i] = info
	}

	return infos, nil
}

func (l *fileInfo) GetModePerm() os.FileMode {
	return l.fileInfo.Mode() & os.ModePerm
}
