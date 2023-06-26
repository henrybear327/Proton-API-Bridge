package proton_api_bridge

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/henrybear327/go-proton-api"

	mathrand "math/rand"
)

// Taken from: https://github.com/rclone/rclone/blob/e43b5ce5e59b5717a9819ff81805dd431f710c10/lib/random/random.go
//
// StringFn create a random string for test purposes using the random
// number generator function passed in.
//
// Do not use these for passwords.
func StringFn(n int, randIntn func(n int) int) string {
	const (
		vowel     = "aeiou"
		consonant = "bcdfghjklmnpqrstvwxyz"
		digit     = "0123456789"
	)
	pattern := []string{consonant, vowel, consonant, vowel, consonant, vowel, consonant, digit}
	out := make([]byte, n)
	p := 0
	for i := range out {
		source := pattern[p]
		p = (p + 1) % len(pattern)
		out[i] = source[randIntn(len(source))]
	}
	return string(out)
}

// String create a random string for test purposes.
//
// Do not use these for passwords.
func RandomString(n int) string {
	return StringFn(n, mathrand.Intn)
}

/* Helper functions */

func createFolder(t *testing.T, ctx context.Context, protonDrive *ProtonDrive, parent, name string) {
	parentLink := protonDrive.RootLink
	if parent != "" {
		targetFolderLink, err := protonDrive.SearchByNameRecursivelyFromRoot(ctx, parent, true)
		if err != nil {
			t.Fatal(err)
		}
		if targetFolderLink == nil {
			t.Fatalf("Folder %v not found", parent)
		}
		parentLink = targetFolderLink
	}
	if parentLink.Type != proton.LinkTypeFolder {
		t.Fatalf("parentLink is not of folder type")
	}

	_, err := protonDrive.CreateNewFolderByID(ctx, parentLink.LinkID, name)
	if err != nil {
		t.Fatal(err)
	}
}

func uploadFile(t *testing.T, ctx context.Context, protonDrive *ProtonDrive, parent, name string, filepath string) {
	parentLink := protonDrive.RootLink
	if parent != "" {
		targetFolderLink, err := protonDrive.SearchByNameRecursivelyFromRoot(ctx, parent, true)
		if err != nil {
			t.Fatal(err)
		}
		if targetFolderLink == nil {
			t.Fatalf("Folder %v not found", parent)
		}
		parentLink = targetFolderLink
	}
	if parentLink.Type != proton.LinkTypeFolder {
		t.Fatalf("parentLink is not of folder type")
	}

	f, err := os.Open(filepath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	info, err := os.Stat(filepath)
	if err != nil {
		t.Fatal(err)
	}

	in := bufio.NewReader(f)

	_, _, err = protonDrive.UploadFileByReader(ctx, parentLink.LinkID, name, info.ModTime(), in)
	if err != nil {
		t.Fatal(err)
	}
}

func downloadFile(t *testing.T, ctx context.Context, protonDrive *ProtonDrive, parent, name string, filepath string) {
	parentLink := protonDrive.RootLink
	if parent != "" {
		targetFolderLink, err := protonDrive.SearchByNameRecursivelyFromRoot(ctx, parent, true)
		if err != nil {
			t.Fatal(err)
		}
		if targetFolderLink == nil {
			t.Fatalf("Folder %v not found", parent)
		}

		parentLink = targetFolderLink
	}
	if parentLink.Type != proton.LinkTypeFolder {
		t.Fatalf("parentLink is not of folder type")
	}

	targetFileLink, err := protonDrive.SearchByNameRecursivelyFromRoot(ctx, name, false)
	if err != nil {
		t.Fatal(err)
	}
	if targetFileLink == nil {
		t.Fatalf("File %v not found", name)
	} else {
		downloadedData, fileSystemAttr, err := protonDrive.DownloadFileByID(ctx, targetFileLink.LinkID)
		if err != nil {
			t.Fatal(err)
		}

		/* Check file metadata */
		if fileSystemAttr == nil {
			t.Fatalf("FileSystemAttr should not be nil")
		} else {
			if len(downloadedData) != int(fileSystemAttr.Size) {
				t.Fatalf("Downloaded file size != uploaded file size: %#v", fileSystemAttr)
			}
		}

		originalData, err := os.ReadFile(filepath)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(downloadedData, originalData) {
			t.Fatalf("Downloaded content is different from the original content")
		}
	}
}

func checkRevisions(protonDrive *ProtonDrive, ctx context.Context, t *testing.T, name string, totalRevisions int) {
	targetFileLink, err := protonDrive.SearchByNameRecursivelyFromRoot(ctx, name, false)
	if err != nil {
		t.Fatal(err)
	}
	if targetFileLink == nil {
		t.Fatalf("File %v not found", name)
	} else {
		revisions, err := protonDrive.c.ListRevisions(ctx, protonDrive.MainShare.ShareID, targetFileLink.LinkID)
		if err != nil {
			t.Fatal(err)
		}

		if len(revisions) != totalRevisions {
			t.Fatalf("Missing revision")
		}
	}
}

// During the integration test, the name much be unique since the link is returned by recursively search for the name from root
func deleteBySearchingFromRoot(t *testing.T, ctx context.Context, protonDrive *ProtonDrive, name string, isFolder bool) {
	targetLink, err := protonDrive.SearchByNameRecursivelyFromRoot(ctx, name, isFolder)
	if err != nil {
		t.Fatal(err)
	}
	if targetLink == nil {
		t.Fatalf("Target %v to be deleted not found", name)
	} else {
		if isFolder {
			err = protonDrive.MoveFolderToTrashByID(ctx, targetLink.LinkID, false)
			if err != nil {
				t.Fatal(err)
			}
		} else {
			err = protonDrive.MoveFileToTrashByID(ctx, targetLink.LinkID)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}

func checkFileListing(t *testing.T, ctx context.Context, protonDrive *ProtonDrive, expectedPaths []string) {
	{
		paths := make([]string, 0)
		err := protonDrive.ListDirectoriesRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, false, -1, 0, true, "", &paths)
		if err != nil {
			t.Fatal(err)
		}

		if len(paths) != len(expectedPaths) {
			t.Fatalf("Total path returned is differs from expected\nReturned %#v\nExpected: %#v\n", paths, expectedPaths)
		}

		for i := range paths {
			if paths[i] != expectedPaths[i] {
				t.Fatalf("The path returned is differs from the path expected\nReturned %#v\nExpected: %#v\n", paths, expectedPaths)
			}
		}
	}

	{
		paths := make([]string, 0)
		err := protonDrive.ListDirectoriesRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, false, -1, 0, false, "", &paths)
		if err != nil {
			t.Fatal(err)
		}

		// transform
		newExpectedPath := make([]string, 0)
		newExpectedPath = append(newExpectedPath, "/root")
		for i := range expectedPaths {
			newExpectedPath = append(newExpectedPath, "/root"+expectedPaths[i])
		}

		if len(paths) != len(newExpectedPath) {
			t.Fatalf("Total path returned is differs from expected\nReturned %#v\nExpected: %#v\n", paths, newExpectedPath)
		}

		for i := range paths {
			if paths[i] != newExpectedPath[i] {
				t.Fatalf("The path returned is differs from the path expected\nReturned %#v\nExpected: %#v\n", paths, newExpectedPath)
			}
		}
	}
}

func moveFolder(t *testing.T, ctx context.Context, protonDrive *ProtonDrive, srcFolderName, dstParentFolderName string) {
	targetSrcFolderLink, err := protonDrive.SearchByNameRecursivelyFromRoot(ctx, srcFolderName, true)
	if err != nil {
		t.Fatal(err)
	}
	targetDestFolderLink, err := protonDrive.SearchByNameRecursivelyFromRoot(ctx, dstParentFolderName, true)
	if err != nil {
		t.Fatal(err)
	}
	if targetSrcFolderLink == nil || targetDestFolderLink == nil {
		t.Fatalf("Folder %s or %s found", srcFolderName, dstParentFolderName)
	} else {
		err := protonDrive.MoveFolder(ctx, targetSrcFolderLink, targetDestFolderLink, srcFolderName)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func moveFile(t *testing.T, ctx context.Context, protonDrive *ProtonDrive, srcFileName, dstParentFolderName string) {
	targetSrcFileLink, err := protonDrive.SearchByNameRecursivelyFromRoot(ctx, srcFileName, false)
	if err != nil {
		t.Fatal(err)
	}
	targetDestFolderLink, err := protonDrive.SearchByNameRecursivelyFromRoot(ctx, dstParentFolderName, true)
	if err != nil {
		t.Fatal(err)
	}
	if targetSrcFileLink == nil || targetDestFolderLink == nil {
		t.Fatalf("File %s or folder %s found", srcFileName, dstParentFolderName)
	} else {
		err := protonDrive.MoveFile(ctx, targetSrcFileLink, targetDestFolderLink, srcFileName)
		if err != nil {
			t.Fatal(err)
		}
	}
}