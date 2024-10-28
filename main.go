package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/wneessen/go-fileperm"
	"gopkg.in/yaml.v3"
)

type Config struct {
	InputPaths       []string `yaml:"input_paths"`
	OutputPath       string   `yaml:"output_path"`
	MoveFiles        bool     `yaml:"move_files"`
	RemoveDuplicates bool     `yaml:"remove_duplicates"`
}

var config Config = Config{
	MoveFiles:        true,
	RemoveDuplicates: false,
}

func main() {
	slog.Info("Starting Iris v0.1.0...")

	slog.Info("Loading config ...")
	if err := loadConfig(&config); err != nil {
		slog.Error(err.Error())
		return
	}

	// Check if output path exists
	if !doesPathExist(config.OutputPath) {
		slog.Error("Output folder does not exist")
		return
	}
	// and if we have the permission to write to it
	permissions, err := fileperm.New(config.OutputPath)
	if err != nil {
		slog.Error(err.Error())
		return
	}
	if !permissions.UserWriteReadable() {
		slog.Error("No write and/or read permission for output folder")
	}

	for _, inputFolderPath := range config.InputPaths {
		slog.Info("Processing folder", "path", inputFolderPath)

		// Check if output path exists
		if !doesPathExist(inputFolderPath) {
			slog.Error("Input folder does not exist")
			return
		}
		// and if we have the permission to write to it
		permissions, err := fileperm.New(inputFolderPath)
		if err != nil {
			slog.Error(err.Error())
			return
		}
		if !permissions.UserWriteReadable() {
			slog.Error("No write and/or read permission for input folder path")
		}

		if err := filepath.WalkDir(inputFolderPath, walk); err != nil {
			slog.Error(err.Error())
		}
	}

	slog.Info("Done!")
}

func loadConfig(configVar *Config) error {
	yamlFile, err := os.ReadFile("config.yaml")
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(yamlFile, configVar)
	if err != nil {
		return err
	}

	return nil
}

func walk(srcFilePath string, srcFileInfo os.DirEntry, err error) error {
	if err != nil {
		slog.Error(err.Error())
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
		slog.Error(err.Error())
		return nil
	}
	defer srcFile.Close()

	// Get file content type, important to distinct images and videos
	fileContentType, err := getFileContentType(srcFile)
	if err != nil {
		slog.Error("Could not get file content type", "srcPath", srcFilePath, "error", err.Error())
		return nil
	}

	// Skip non image and video files
	supportedFileContentTypes := []string{"image/jpeg", "video/mp4"}
	if !slices.Contains(supportedFileContentTypes, fileContentType) {
		slog.Warn("File is not a image or video", "srcPath", srcFilePath, "fileContentTrypes", fileContentType)
		return nil
	}

	destFilePath := destFilePath{
		rootPath:      config.OutputPath,
		fileExtension: filepath.Ext(srcFilePath),
	}

	destFilePath.creationTime, err = getCreationTimeFromMedia(srcFile, srcFilePath, fileContentType)
	if err != nil {
		slog.Error("Could not get media creation time", "srcPath", srcFilePath)
		return nil
	}

	// Create the folder if it doesn't exist
	folderPath := filepath.Dir(destFilePath.generate())
	if !doesPathExist(folderPath) {
		err := os.MkdirAll(folderPath, os.ModePerm)
		if err != nil {
			slog.Error("Could not create folder", "folderPath", folderPath, "error", err.Error())
			// Stop completely since this likely also affects other files
			return filepath.SkipAll
		}
	}

	// File exists, check if they are the same
	for doesPathExist(destFilePath.generate()) {
		srcFileHash, err := getFileHash(srcFile)
		if err != nil {
			slog.Error("Could not get file hash", "path", srcFilePath, "error", err.Error())
			return nil
		}

		// Get hash of the existing file
		destFile, err := os.Open(destFilePath.generate())
		if err != nil {
			slog.Error("Could not open file", "destPath", destFilePath.generate(), "error", err.Error())
			return nil
		}
		destFileHash, err := getFileHash(destFile)
		if err != nil {
			slog.Error("Could not get file hash", "destPath", destFilePath.generate(), "error", err.Error())
			return nil
		}
		destFile.Close()

		if srcFileHash == destFileHash {
			// Skip if they are the same
			slog.Warn("File already exists", "destPath", destFilePath.generate())

			// Remove duplicated file if configured
			if config.RemoveDuplicates {
				err := os.Remove(srcFilePath)
				if err != nil {
					slog.Error("Could not remove file", "srcPath", srcFilePath, "error", err.Error())
				}
			}

			return nil
		} else {
			// Try another name if they are different
			slog.Warn("Different file with same path found", "destPath", destFilePath.generate())
			destFilePath.number++
		}
	}

	// Copy or move the file
	err = copyFile(srcFile, destFilePath.generate())
	if err != nil {
		slog.Error("Could not copy file", "srcPath", srcFilePath, "error", err.Error())
		return nil
	}
	if config.MoveFiles {
		err = os.Remove(srcFilePath)
		if err != nil {
			slog.Error("Could not remove file", "srcPath", srcFilePath, "error", err.Error())
			return nil
		}
	}

	return nil
}

func getCreationTimeFromMedia(file *os.File, filePath string, fileContentType string) (time.Time, error) {
	// Get creation time, important to distinct images and videos since they have different metadata
	if strings.HasPrefix(fileContentType, "image") {
		if creationTime, err := getImageCreationTime(file); err == nil {
			return creationTime, nil
		}
	}
	if strings.HasPrefix(fileContentType, "video") {
		if creationTime, err := getVideoCreationTime(file); err == nil {
			return creationTime, nil
		}
	}

	// Try to get date from the filename if the above don't work
	srcFileName := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	// 2006: year, 01: month, 02: day, 15: hour, 04: minute, 05: second
	possibleTimeFormats := []string{
		"2006-01-02_15-04-05",
		"IMG_20060102_150405",
		"PXL_20060102_150405",
		"IMG-20060102",
		"signal-2006-01-02-15-04-05",
		"image_20060102150405",
		"20060102_150405",
	}
	for _, format := range possibleTimeFormats {
		// Try to remove some random stuff at the end of some image names
		if len(srcFileName) < len(format) {
			continue
		}
		cleanSrcFileName := srcFileName[:len(format)]

		if creationTime, err := time.Parse(format, cleanSrcFileName); err == nil {
			return creationTime, nil
		}
	}

	return time.Time{}, errors.New("could not determine media creation time")
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
