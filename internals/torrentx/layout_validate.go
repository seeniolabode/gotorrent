package torrentx

import (
	"fmt"
	"strings"
)

type pathType int

const (
	directoryPath pathType = iota
	filePath
)

func validateFileLayout(layout []FileLayout) error {
	paths := make(map[string]pathType)

	for i, file := range layout {
		filePaths := getPathsFromPath(file.Path)

		for j, path := range filePaths {
			isFinalPath := j == len(filePaths)-1

			existingType, exists := paths[path]

			if isFinalPath {
				// This path needs to be a file.
				if exists {
					return fmt.Errorf(
						"colliding path at layout(%d): %s already exists",
						i,
						path,
					)
				}

				paths[path] = filePath
				continue
			}

			// This path needs to be a directory.
			if exists && existingType == filePath {
				return fmt.Errorf(
					"colliding path at layout(%d): %s is already a file",
					i,
					path,
				)
			}

			paths[path] = directoryPath
		}
	}

	return nil
}

func getPathsFromPath(path string) []string {
	pathParts := strings.Split(path, "/")
	fullPaths := make([]string, 0, len(pathParts))

	for i := range pathParts {
		fullPaths = append(
			fullPaths,
			strings.Join(pathParts[:i+1], "/"),
		)
	}

	return fullPaths
}
