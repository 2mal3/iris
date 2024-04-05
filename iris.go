package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	InputPaths []string `yaml:"input_paths"`
	OutputPath string   `yaml:"output_path"`
	MoveFiles  bool     `yaml:"move_files"`
}

var config Config = Config{
	MoveFiles: true,
}

func main() {
	slog.Info("Starting Iris...")

	slog.Info("Loading config ...")
	if err := loadConfig(&config); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	for _, path := range config.InputPaths {
		slog.Info("Processing folder", "path", path)
		if err := filepath.WalkDir(path, walk); err != nil {
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
		slog.Error("Could not get file content type", "path", srcFilePath, "error", err.Error())
		return nil
	}

	// Skip non image and video files
	supportedFileContentTypes := []string{"image/jpeg", "video/mp4"}
	if !slices.Contains(supportedFileContentTypes, fileContentType) {
		slog.Warn("File is not a image or video", "path", srcFilePath, "fileContentTrypes", fileContentType)
		return nil
	}

	destFilePath := destFilePath{
		rootPath:      config.OutputPath,
		fileExtension: filepath.Ext(srcFilePath),
	}

	// Get creation time, important to distinct images and videos since they have different metadata
	if strings.HasPrefix(fileContentType, "image") {
		destFilePath.creationTime, err = getImageCreationTime(srcFile)
		if err != nil {
			slog.Warn("Could not get image creation time from metadata", "path", srcFilePath, "error", err.Error())
		}
	}
	if strings.HasPrefix(fileContentType, "video") {
		destFilePath.creationTime, err = getVideoCreationTime(srcFile)
		if err != nil {
			slog.Warn("Could not get video creation time from metadata", "path", srcFilePath, "error", err.Error())
		}
	}
	// Try to get date from the filename if the above don't work
	if destFilePath.creationTime.IsZero() {
		srcFileName := strings.TrimSuffix(filepath.Base(srcFilePath), filepath.Ext(srcFilePath))
		possibleTimeFormats := []string{
			"2006-01-02_15-04-05",
			"IMG_20060102_150405",
			"PXL_20060102_150405",
			"IMG-20060102",
		}
		for _, format := range possibleTimeFormats {
			cleanSrcFileName := srcFileName[:len(format)] // Remove some random stuff at the end of some image names
			destFilePath.creationTime, err = time.Parse(format, cleanSrcFileName)
			if err == nil {
				break
			}
		}
	}
	if destFilePath.creationTime.IsZero() {
		slog.Error("Could not determine creation time", "path", srcFilePath)
		return nil
	}

	// Create the folder if it doesn't exist
	folderPath := filepath.Dir(destFilePath.generate())
	if doesPathExist(folderPath) {
		err := os.MkdirAll(folderPath, os.ModePerm)
		if err != nil {
			slog.Error("Could not create folder", "path", folderPath, "error", err.Error())
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
			slog.Error("Could not open file", "path", destFilePath.generate(), "error", err.Error())
			return nil
		}
		destFileHash, err := getFileHash(destFile)
		if err != nil {
			slog.Error("Could not get file hash", "path", destFilePath.generate(), "error", err.Error())
			return nil
		}
		destFile.Close()

		if srcFileHash == destFileHash {
			// Skip if they are the same
			slog.Warn("File already exists", "path", destFilePath.generate())
			return nil
		} else {
			// Try another name if they are different
			slog.Warn("Different file with same path found", "path", destFilePath.generate())
			destFilePath.number++
		}
	}

	// Copy or move the file
	err = copyFile(srcFile, destFilePath.generate())
	if err != nil {
		slog.Error("Could not copy file", "path", srcFilePath, "error", err.Error())
		return nil
	}
	if config.MoveFiles {
		err = os.Remove(srcFilePath)
		if err != nil {
			slog.Error("Could not remove file", "path", srcFilePath, "error", err.Error())
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
