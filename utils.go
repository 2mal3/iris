package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/rwcarlsen/goexif/exif"
	"gopkg.in/vansante/go-ffprobe.v2"
)

func doesPathExist(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	} else if os.IsNotExist(err) {
		return false
	} else {
		return false
	}
}

func getFileHash(file *os.File) (string, error) {
	// Reset the file's read position because we want to use the same file object again
	defer file.Seek(0, 0)

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func copyFile(sourceFile *os.File, destPath string) error {
	// Reset the file's read position because we want to use the same file object again
	defer sourceFile.Seek(0, 0)

	desFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer desFile.Close()

	_, err = io.Copy(desFile, sourceFile)
	if err != nil {
		return err
	}

	return nil
}

func getFileContentType(file *os.File) (string, error) {
	// Reset the file's read position because we want to use the same file object again
	defer file.Seek(0, 0)

	// Read the first 512 bytes of the file
	buffer := make([]byte, 512)
	_, err := file.Read(buffer)
	if err != nil {
		return "", err
	}

	// Detect the content type based on the file header
	contentType := http.DetectContentType(buffer)
	return contentType, nil
}

func getImageCreationTime(file *os.File) (time.Time, error) {
	// Reset the file's read position because we want to use the same file object again
	defer file.Seek(0, 0)

	data, err := exif.Decode(file)
	if err != nil {
		return time.Time{}, err
	}

	creationTime, err := data.DateTime()
	if err != nil {
		return time.Time{}, err
	}

	return creationTime, nil
}

func getVideoCreationTime(file *os.File) (time.Time, error) {
	// Reset the file's read position because we want to use the same file object again
	defer file.Seek(0, 0)

	ctx, cancelFn := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFn()

	data, err := ffprobe.ProbeReader(ctx, file)
	if err != nil {
		return time.Time{}, err
	}

	creationTimeString, err := data.Format.TagList.GetString("creation_time")
	if err != nil {
		return time.Time{}, err
	}
	creationTime, err := time.Parse("2006-01-02T15:04:05.000000Z", creationTimeString)
	if err != nil {
		return time.Time{}, err
	}

	return creationTime, nil
}
