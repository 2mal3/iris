package pkg

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/wneessen/go-fileperm"
)

var config Config

type Config struct {
	InputPaths       []string
	OutputPath       string
	MoveFiles        bool
	RemoveDuplicates bool
}

func Main(inputConfig Config) error {
	config = inputConfig

	fmt.Fprintf(os.Stderr, "Starting Iris ...\n")

	// Check if output path exists
	if !doesPathExist(config.OutputPath) {
		return errors.New("Output folder does not exist")
	}
	// and if we have the permission to write to it
	permissions, err := fileperm.New(config.OutputPath)
	if err != nil {
		return err
	}
	if !permissions.UserWriteReadable() {
		return errors.New("No write and/or read permission for output folder")
	}

	for _, inputFolderPath := range config.InputPaths {
		fmt.Fprintf(os.Stderr, "Processing folder: %s\n", inputFolderPath)

		// Check if output path exists
		if !doesPathExist(inputFolderPath) {
			fmt.Fprintln(os.Stderr, "Input folder does not exist", inputFolderPath)
			continue
		}
		// and if we have the permission to write to it
		permissions, err := fileperm.New(inputFolderPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "File permission error", err)
			continue
		}
		if !permissions.UserWriteReadable() {
			fmt.Fprintln(os.Stderr, "No write and/or read permission for input folder path", inputFolderPath)
			continue
		}

		if err := filepath.WalkDir(inputFolderPath, walk); err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
	}

	return nil
}

func walk(srcFilePath string, srcFileInfo os.DirEntry, err error) error {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err.Error())
		return nil
	}

	if srcFileInfo.IsDir() {
		// Skip hidden folders
		if strings.HasPrefix(srcFileInfo.Name(), ".") {
			return filepath.SkipDir
			// Don't process folders
		} else {
			return nil
		}
	}

	// Skip non regular files
	if !srcFileInfo.Type().IsRegular() {
		return nil
	}

	// Skip hidden files
	if strings.HasPrefix(srcFileInfo.Name(), ".") {
		return nil
	}

	// Open srcFile
	srcFile, err := os.Open(srcFilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err.Error())
		return nil
	}
	defer srcFile.Close()

	// Get file content type, important to distinct images and videos
	fileContentType, err := getFileContentType(srcFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not get file content type: %s error=%v\n", srcFilePath, err.Error())
		return nil
	}

	// Skip non image and video files
	supportedFileContentTypes := []string{"image/jpeg", "video/mp4"}
	if !slices.Contains(supportedFileContentTypes, fileContentType) {
		fmt.Fprintf(os.Stderr, "File is not a image or video: %s\n", srcFilePath)
		return nil
	}

	destFilePath := destFilePath{
		rootPath:      config.OutputPath,
		fileExtension: filepath.Ext(srcFilePath),
	}

	// Get creation time, important to distinct images and videos since they have different metadata
	if strings.HasPrefix(fileContentType, "image") {
		destFilePath.creationTime, _ = getImageCreationTime(srcFile)
	}
	if strings.HasPrefix(fileContentType, "video") {
		destFilePath.creationTime, _ = getVideoCreationTime(srcFile)
	}
	// Try to get date from the filename if the above don't work
	if destFilePath.creationTime.IsZero() {
		srcFileName := strings.TrimSuffix(filepath.Base(srcFilePath), filepath.Ext(srcFilePath))
		// 2006: year, 01: month, 02: day, 15: hour, 04: minute, 05: second
		possibleTimeFormats := []string{
			"2006-01-02_15-04-05",
			"IMG_20060102_150405",
			"PXL_20060102_150405",
			"IMG-20060102",
			"signal-2006-01-02-15-04-05",
			"image_20060102150405",
		}
		for _, format := range possibleTimeFormats {
			// Try to remove some random stuff at the end of some image names
			if len(srcFileName) < len(format) {
				continue
			}
			cleanSrcFileName := srcFileName[:len(format)]

			destFilePath.creationTime, err = time.Parse(format, cleanSrcFileName)
			if err == nil {
				break
			}
		}
	}
	if destFilePath.creationTime.IsZero() {
		fmt.Fprintf(os.Stderr, "Could not determine creation time srcPath=%v\n", srcFilePath)
		return nil
	}

	// Create the folder if it doesn't exist
	folderPath := filepath.Dir(destFilePath.generate())
	if !doesPathExist(folderPath) {
		err := os.MkdirAll(folderPath, os.ModePerm)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not create folder folderPath=%v error=%v\n", folderPath, err.Error())
			// Stop completely since this likely also affects other files
			return filepath.SkipAll
		}
	}

	// File exists, check if they are the same
	for doesPathExist(destFilePath.generate()) {
		srcFileHash, err := getFileHash(srcFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not get file hash path=%v error=%v\n", srcFilePath, err.Error())
			return nil
		}

		// Get hash of the existing file
		destFile, err := os.Open(destFilePath.generate())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not open file destPath=%v error=%v\n", destFilePath.generate(), err.Error())
			return nil
		}
		destFileHash, err := getFileHash(destFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not get file hash destPath=%v error=%v\n", destFilePath.generate(), err.Error())
			return nil
		}
		destFile.Close()

		if srcFileHash == destFileHash {
			// Skip if they are the same
			fmt.Fprintf(os.Stderr, "File already exists destPath=%v\n", destFilePath.generate())

			// Remove duplicated file if configured
			if config.RemoveDuplicates {
				err := os.Remove(srcFilePath)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Could not remove file srcPath=%v error=%v\n", srcFilePath, err.Error())
				}
			}

			return nil
		} else {
			// Try another name if they are different
			fmt.Fprintf(os.Stderr, "Different file with same path found destPath=%v\n", destFilePath.generate())
			destFilePath.number++
		}
	}

	// Copy or move the file
	err = copyFile(srcFile, destFilePath.generate())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not copy file srcPath=%v error=%v\n", srcFilePath, err.Error())
		return nil
	}
	if config.MoveFiles {
		err = os.Remove(srcFilePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not remove file srcPath=%v error=%v\n", srcFilePath, err.Error())
			return nil
		}
	}

	return nil
}

type destFilePath struct {
	rootPath      string
	creationTime  time.Time
	number        int
	fileExtension string
}

func (d *destFilePath) generate() string {
	// Determine folder name to put the file in
	var yearQuarter string
	if d.creationTime.Month() <= 2 {
		yearQuarter = fmt.Sprintf("%d-4", d.creationTime.Year()-1)
	} else if d.creationTime.Month() <= 5 {
		yearQuarter = fmt.Sprintf("%d-1", d.creationTime.Year())
	} else if d.creationTime.Month() <= 8 {
		yearQuarter = fmt.Sprintf("%d-2", d.creationTime.Year())
	} else if d.creationTime.Month() <= 11 {
		yearQuarter = fmt.Sprintf("%d-3", d.creationTime.Year())
	} else if d.creationTime.Month() == 12 {
		yearQuarter = fmt.Sprintf("%d-4", d.creationTime.Year())
	}

	var numberSuffix string
	if d.number != 0 {
		numberSuffix = fmt.Sprintf("_%d", d.number)
	}

	destFilePath := filepath.Join(
		d.rootPath,
		yearQuarter,
		d.creationTime.Format("2006-01-02_15-04-05")+numberSuffix+d.fileExtension,
	)

	return destFilePath
}
