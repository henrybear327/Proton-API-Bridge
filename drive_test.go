package proton_api_bridge

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/henrybear327/Proton-API-Bridge/common"
	"github.com/henrybear327/Proton-API-Bridge/utility"
)

/* Helper functions */
func setup(t *testing.T) (context.Context, context.CancelFunc, *ProtonDrive) {
	utility.SetupLog()

	config := common.NewConfigForIntegrationTests()

	{
		// pre-condition check
		if config.DestructiveIntegrationTest == false {
			t.Fatalf("CAUTION: the integration test requires a clean proton drive")
		}
		if config.EmptyTrashAfterIntegrationTest == false {
			t.Fatalf("CAUTION: the integration test requires cleaning up the drive after running the tests")
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	protonDrive, err := NewProtonDrive(ctx, config)
	if err != nil {
		t.Fatal(err)
	}

	err = protonDrive.EmptyRootFolder(ctx)
	if err != nil {
		t.Fatal(err)
	}

	err = protonDrive.EmptyTrash(ctx)
	if err != nil {
		t.Fatal(err)
	}

	return ctx, cancel, protonDrive
}

func tearDown(t *testing.T, ctx context.Context, protonDrive *ProtonDrive) {
	if protonDrive.Config.EmptyTrashAfterIntegrationTest {
		err := protonDrive.EmptyTrash(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}
}

/* Integration Tests */
func TestCreateAndDeleteFolderAtRoot(t *testing.T) {
	ctx, cancel, protonDrive := setup(t)
	t.Cleanup(func() {
		defer cancel()
		defer tearDown(t, ctx, protonDrive)
	})

	{
		/* Create folder tmp */
		_, err := protonDrive.CreateNewFolderByID(ctx, protonDrive.RootLink.LinkID, "tmp")
		if err != nil {
			t.Fatal(err)
		}

		paths := make([]string, 0)
		err = protonDrive.ListDirectoriesRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, false, -1, 0, true, "", &paths)
		if err != nil {
			t.Fatal(err)
		}

		if len(paths) != 1 {
			t.Fatalf("Total path returned is differs from expected: %#v", paths)
		}
		if paths[0] != "/tmp" {
			t.Fatalf("Wrong folder created")
		}

		paths = make([]string, 0)
		err = protonDrive.ListDirectoriesRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, false, -1, 0, false, "", &paths)
		if err != nil {
			t.Fatal(err)
		}

		if len(paths) != 2 {
			t.Fatalf("Total path returned is differs from expected: %#v", paths)
		}
		if paths[0] != "/root" {
			t.Fatalf("Wrong root folder")
		}
		if paths[1] != "/root/tmp" {
			t.Fatalf("Wrong folder created")
		}
	}

	{
		/* Delete folder tmp */
		targetFolderLink, err := protonDrive.SearchByNameRecursivelyFromRoot(ctx, "tmp", true)
		if err != nil {
			t.Fatal(err)
		}
		if targetFolderLink == nil {
			t.Fatalf("Folder tmp not found")
		} else {
			err = protonDrive.MoveFolderToTrashByID(ctx, targetFolderLink.LinkID, false)
			if err != nil {
				t.Fatal(err)
			}
		}

		paths := make([]string, 0)
		err = protonDrive.ListDirectoriesRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, false, -1, 0, false, "", &paths)
		if err != nil {
			t.Fatal(err)
		}

		if len(paths) != 1 {
			t.Fatalf("Total path returned is differs from expected: %#v", paths)
		}
		if paths[0] != "/root" {
			t.Fatalf("Wrong root folder")
		}
	}
}

func TestUploadAndDownloadAndDeleteAFileAtRoot(t *testing.T) {
	ctx, cancel, protonDrive := setup(t)
	t.Cleanup(func() {
		defer cancel()
		defer tearDown(t, ctx, protonDrive)
	})

	{
		/* Upload a file integrationTestImage.png */
		f, err := os.Open("testcase/integrationTestImage.png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		info, err := os.Stat("testcase/integrationTestImage.png")
		if err != nil {
			t.Fatal(err)
		}

		in := bufio.NewReader(f)

		_, _, err = protonDrive.UploadFileByReader(ctx, protonDrive.RootLink.LinkID, "integrationTestImage.png", info.ModTime(), in)
		if err != nil {
			t.Fatal(err)
		}

		paths := make([]string, 0)
		err = protonDrive.ListDirectoriesRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, false, -1, 0, true, "", &paths)
		if err != nil {
			t.Fatal(err)
		}

		if len(paths) != 1 {
			t.Fatalf("Total path returned is not as expected: %#v", paths)
		}
		if paths[0] != "/integrationTestImage.png" {
			t.Fatalf("Wrong file name decrypted")
		}

		paths = make([]string, 0)
		err = protonDrive.ListDirectoriesRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, false, -1, 0, false, "", &paths)
		if err != nil {
			t.Fatal(err)
		}

		if len(paths) != 2 {
			t.Fatalf("Total path returned is differs from expected: %#v", paths)
		}
		if paths[0] != "/root" {
			t.Fatalf("Wrong root folder")
		}
		if paths[1] != "/root/integrationTestImage.png" {
			t.Fatalf("Wrong file name decrypted")
		}
	}

	{
		/* Download a file integrationTestImage.png */
		targetFileLink, err := protonDrive.SearchByNameRecursivelyFromRoot(ctx, "integrationTestImage.png", false)
		if err != nil {
			t.Fatal(err)
		}
		if targetFileLink == nil {
			t.Fatalf("File integrationTestImage.png not found")
		} else {
			{
				_, err := protonDrive.SearchByNameInFolder(ctx, targetFileLink, "integrationTestImage.png", true, false)
				if err != ErrLinkTypeMustToBeFolderType {
					t.Fatalf("Wrong error message being returned")
				}
			}

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

			originalData, err := os.ReadFile("testcase/integrationTestImage.png")
			if err != nil {
				t.Fatal(err)
			}

			if bytes.Equal(downloadedData, originalData) == false {
				t.Fatalf("Downloaded content is different from the original content")
			}
		}
	}

	{
		/* Add a revision */
		{
			/* Upload a file integrationTestImage.png */
			f, err := os.Open("testcase/integrationTestImage.png")
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()

			info, err := os.Stat("testcase/integrationTestImage.png")
			if err != nil {
				t.Fatal(err)
			}

			in := bufio.NewReader(f)

			_, _, err = protonDrive.UploadFileByReader(ctx, protonDrive.RootLink.LinkID, "integrationTestImage.png", info.ModTime(), in)
			if err != nil {
				t.Fatal(err)
			}

			paths := make([]string, 0)
			err = protonDrive.ListDirectoriesRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, false, -1, 0, true, "", &paths)
			if err != nil {
				t.Fatal(err)
			}

			if len(paths) != 1 {
				t.Fatalf("Total path returned is not as expected: %#v", paths)
			}
			if paths[0] != "/integrationTestImage.png" {
				t.Fatalf("Wrong file name decrypted")
			}

			paths = make([]string, 0)
			err = protonDrive.ListDirectoriesRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, false, -1, 0, false, "", &paths)
			if err != nil {
				t.Fatal(err)
			}

			if len(paths) != 2 {
				t.Fatalf("Total path returned is differs from expected: %#v", paths)
			}
			if paths[0] != "/root" {
				t.Fatalf("Wrong root folder")
			}
			if paths[1] != "/root/integrationTestImage.png" {
				t.Fatalf("Wrong file name decrypted")
			}
		}

		{
			/* Check total revisions */
			targetFileLink, err := protonDrive.SearchByNameRecursivelyFromRoot(ctx, "integrationTestImage.png", false)
			if err != nil {
				t.Fatal(err)
			}
			if targetFileLink == nil {
				t.Fatalf("File integrationTestImage.png not found")
			} else {
				revisions, err := protonDrive.c.ListRevisions(ctx, protonDrive.MainShare.ShareID, targetFileLink.LinkID)
				if err != nil {
					t.Fatal(err)
				}

				if len(revisions) != 2 {
					t.Fatalf("Missing revision")
				}
			}
		}

		{
			/* Download a file integrationTestImage.png */
			targetFileLink, err := protonDrive.SearchByNameRecursivelyFromRoot(ctx, "integrationTestImage.png", false)
			if err != nil {
				t.Fatal(err)
			}
			if targetFileLink == nil {
				t.Fatalf("File integrationTestImage.png not found")
			} else {
				{
					_, err := protonDrive.SearchByNameInFolder(ctx, targetFileLink, "integrationTestImage.png", true, false)
					if err != ErrLinkTypeMustToBeFolderType {
						t.Fatalf("Wrong error message being returned")
					}
				}

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

				originalData, err := os.ReadFile("testcase/integrationTestImage.png")
				if err != nil {
					t.Fatal(err)
				}

				if bytes.Equal(downloadedData, originalData) == false {
					t.Fatalf("Downloaded content is different from the original content")
				}
			}
		}
	}

	{
		/* Delete a file integrationTestImage.png */
		targetFileLink, err := protonDrive.SearchByNameRecursivelyFromRoot(ctx, "integrationTestImage.png", false)
		if err != nil {
			t.Fatal(err)
		}
		if targetFileLink == nil {
			t.Fatalf("File integrationTestImage.png not found")
		} else {
			err = protonDrive.MoveFileToTrashByID(ctx, targetFileLink.LinkID)
			if err != nil {
				t.Fatal(err)
			}
		}

		paths := make([]string, 0)
		err = protonDrive.ListDirectoriesRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, false, -1, 0, false, "", &paths)
		if err != nil {
			t.Fatal(err)
		}

		if len(paths) != 1 {
			t.Fatalf("Total path returned is differs from expected: %#v", paths)
		}
		if paths[0] != "/root" {
			t.Fatalf("Wrong root folder")
		}
	}
}

func TestUploadAndDeleteAnEmptyFileAtRoot(t *testing.T) {
	ctx, cancel, protonDrive := setup(t)
	t.Cleanup(func() {
		defer cancel()
		defer tearDown(t, ctx, protonDrive)
	})

	{
		/* Upload a file integrationTestImage.png */
		_, _, err := protonDrive.UploadFileByPath(ctx, protonDrive.RootLink, "empty.txt", "testcase/empty.txt")
		if err != nil {
			t.Fatal(err)
		}

		paths := make([]string, 0)
		err = protonDrive.ListDirectoriesRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, false, -1, 0, true, "", &paths)
		if err != nil {
			t.Fatal(err)
		}

		if len(paths) != 1 {
			t.Fatalf("Total path returned is differs from expected: %#v", paths)
		}
		if paths[0] != "/empty.txt" {
			t.Fatalf("Wrong file name decrypted")
		}

		paths = make([]string, 0)
		err = protonDrive.ListDirectoriesRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, false, -1, 0, false, "", &paths)
		if err != nil {
			t.Fatal(err)
		}

		if len(paths) != 2 {
			t.Fatalf("Total path returned is differs from expected: %#v", paths)
		}
		if paths[0] != "/root" {
			t.Fatalf("Wrong root folder")
		}
		if paths[1] != "/root/empty.txt" {
			t.Fatalf("Wrong file name decrypted")
		}
	}

	{
		/* Download a file empty.txt */
		targetFileLink, err := protonDrive.SearchByNameRecursivelyFromRoot(ctx, "empty.txt", false)
		if err != nil {
			t.Fatal(err)
		}
		if targetFileLink == nil {
			t.Fatalf("File empty.txt not found")
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

			originalData, err := os.ReadFile("testcase/empty.txt")
			if err != nil {
				t.Fatal(err)
			}

			if bytes.Equal(downloadedData, originalData) == false {
				t.Fatalf("Downloaded content is different from the original content")
			}
		}
	}

	{
		/* Add a revision */
		{
			/* Upload a file integrationTestImage.png */
			_, _, err := protonDrive.UploadFileByPath(ctx, protonDrive.RootLink, "empty.txt", "testcase/empty.txt")
			if err != nil {
				t.Fatal(err)
			}

			paths := make([]string, 0)
			err = protonDrive.ListDirectoriesRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, false, -1, 0, true, "", &paths)
			if err != nil {
				t.Fatal(err)
			}

			if len(paths) != 1 {
				t.Fatalf("Total path returned is differs from expected: %#v", paths)
			}
			if paths[0] != "/empty.txt" {
				t.Fatalf("Wrong file name decrypted")
			}

			paths = make([]string, 0)
			err = protonDrive.ListDirectoriesRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, false, -1, 0, false, "", &paths)
			if err != nil {
				t.Fatal(err)
			}

			if len(paths) != 2 {
				t.Fatalf("Total path returned is differs from expected: %#v", paths)
			}
			if paths[0] != "/root" {
				t.Fatalf("Wrong root folder")
			}
			if paths[1] != "/root/empty.txt" {
				t.Fatalf("Wrong file name decrypted")
			}
		}

		{
			/* Check total revisions */
			targetFileLink, err := protonDrive.SearchByNameRecursivelyFromRoot(ctx, "empty.txt", false)
			if err != nil {
				t.Fatal(err)
			}
			if targetFileLink == nil {
				t.Fatalf("File empty.txt not found")
			} else {
				revisions, err := protonDrive.c.ListRevisions(ctx, protonDrive.MainShare.ShareID, targetFileLink.LinkID)
				if err != nil {
					t.Fatal(err)
				}

				if len(revisions) != 2 {
					t.Fatalf("Missing revision")
				}
			}
		}

		{
			/* Download a file empty.txt */
			targetFileLink, err := protonDrive.SearchByNameRecursivelyFromRoot(ctx, "empty.txt", false)
			if err != nil {
				t.Fatal(err)
			}
			if targetFileLink == nil {
				t.Fatalf("File empty.txt not found")
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

				originalData, err := os.ReadFile("testcase/empty.txt")
				if err != nil {
					t.Fatal(err)
				}

				if bytes.Equal(downloadedData, originalData) == false {
					t.Fatalf("Downloaded content is different from the original content")
				}
			}
		}
	}

	{
		/* Delete a file empty.txt */
		targetFileLink, err := protonDrive.SearchByNameRecursivelyFromRoot(ctx, "empty.txt", false)
		if err != nil {
			t.Fatal(err)
		}
		if targetFileLink == nil {
			t.Fatalf("File empty.txt not found")
		} else {
			err = protonDrive.MoveFileToTrashByID(ctx, targetFileLink.LinkID)
			if err != nil {
				t.Fatal(err)
			}
		}

		paths := make([]string, 0)
		err = protonDrive.ListDirectoriesRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, false, -1, 0, false, "", &paths)
		if err != nil {
			t.Fatal(err)
		}

		if len(paths) != 1 {
			t.Fatalf("Total path returned is differs from expected: %#v", paths)
		}
		if paths[0] != "/root" {
			t.Fatalf("Wrong root folder")
		}
	}
}

func TestUploadAndDownloadAndDeleteAFileAtAFolderOneLevelFromRoot(t *testing.T) {
	ctx, cancel, protonDrive := setup(t)
	t.Cleanup(func() {
		defer cancel()
		defer tearDown(t, ctx, protonDrive)
	})

	{
		/* Upload a file integrationTestImage.png */
		_, err := protonDrive.CreateNewFolder(ctx, protonDrive.RootLink, "tmp")
		if err != nil {
			t.Fatal(err)
		}

		targetFolderLink, err := protonDrive.SearchByNameRecursivelyFromRoot(ctx, "tmp", true)
		if err != nil {
			t.Fatal(err)
		}
		if targetFolderLink == nil {
			t.Fatalf("Folder tmp not found")
		}
		_, _, err = protonDrive.UploadFileByPath(ctx, targetFolderLink, "integrationTestImage.png", "testcase/integrationTestImage.png")
		if err != nil {
			t.Fatal(err)
		}

		paths := make([]string, 0)
		err = protonDrive.ListDirectoriesRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, false, -1, 0, true, "", &paths)
		if err != nil {
			t.Fatal(err)
		}

		if len(paths) != 2 {
			t.Fatalf("Total path returned is differs from expected: %#v", paths)
		}
		if paths[0] != "/tmp" {
			t.Fatalf("Wrong folder name decrypted")
		}
		if paths[1] != "/tmp/integrationTestImage.png" {
			t.Fatalf("Wrong file name decrypted")
		}

		paths = make([]string, 0)
		err = protonDrive.ListDirectoriesRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, false, -1, 0, false, "", &paths)
		if err != nil {
			t.Fatal(err)
		}

		if len(paths) != 3 {
			t.Fatalf("Total path returned is differs from expected: %#v", paths)
		}
		if paths[0] != "/root" {
			t.Fatalf("Wrong root folder")
		}
		if paths[1] != "/root/tmp" {
			t.Fatalf("Wrong folder name decrypted")
		}
		if paths[2] != "/root/tmp/integrationTestImage.png" {
			t.Fatalf("Wrong file name decrypted")
		}
	}

	{
		/* Download a file integrationTestImage.png */
		targetFileLink, err := protonDrive.SearchByNameRecursivelyFromRoot(ctx, "integrationTestImage.png", false)
		if err != nil {
			t.Fatal(err)
		}

		{
			targetFolderLink, err := protonDrive.SearchByNameRecursivelyFromRoot(ctx, "tmp", true)
			if err != nil {
				t.Fatal(err)
			}
			if targetFolderLink == nil {
				t.Fatalf("Folder tmp not found")
			} else {
				fileLink, err := protonDrive.SearchByNameInFolder(ctx, targetFolderLink, "integrationTestImage.png", true, false)
				if err != nil {
					t.Fatal(err)
				}

				if fileLink.LinkID != targetFileLink.LinkID {
					t.Fatalf("Wrong file being returned")
				}
			}

			targetFileLink2, err := protonDrive.SearchByNameRecursively(ctx, targetFolderLink, "integrationTestImage.png", false)
			if err != nil {
				t.Fatal(err)
			}

			if targetFileLink.LinkID != targetFileLink2.LinkID {
				t.Fatalf("SearchByNameRecursively is broken")
			}
		}

		if targetFileLink == nil {
			t.Fatalf("File integrationTestImage.png not found")
		} else {
			downloadedData, fileSystemAttr, err := protonDrive.DownloadFile(ctx, targetFileLink)
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

			originalData, err := os.ReadFile("testcase/integrationTestImage.png")
			if err != nil {
				t.Fatal(err)
			}

			if bytes.Equal(downloadedData, originalData) == false {
				t.Fatalf("Downloaded content is different from the original content")
			}
		}
	}

	{
		/* Delete a file integrationTestImage.png */
		targetFileLink, err := protonDrive.SearchByNameRecursivelyFromRoot(ctx, "integrationTestImage.png", false)
		if err != nil {
			t.Fatal(err)
		}
		if targetFileLink == nil {
			t.Fatalf("File integrationTestImage.png not found")
		} else {
			err = protonDrive.MoveFileToTrashByID(ctx, targetFileLink.LinkID)
			if err != nil {
				t.Fatal(err)
			}
		}

		paths := make([]string, 0)
		err = protonDrive.ListDirectoriesRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, false, -1, 0, false, "", &paths)
		if err != nil {
			t.Fatal(err)
		}

		if len(paths) != 2 {
			t.Fatalf("Total path returned is differs from expected: %#v", paths)
		}
		if paths[0] != "/root" {
			t.Fatalf("Wrong root folder")
		}
		if paths[1] != "/root/tmp" {
			t.Fatalf("Wrong tmp folder")
		}
	}

	{
		/* Delete a folder tmp */
		targetFolderLink, err := protonDrive.SearchByNameRecursivelyFromRoot(ctx, "tmp", true)
		if err != nil {
			t.Fatal(err)
		}
		if targetFolderLink == nil {
			t.Fatalf("Folder tmp not found")
		} else {
			err = protonDrive.MoveFolderToTrashByID(ctx, targetFolderLink.LinkID, false)
			if err != nil {
				t.Fatal(err)
			}
		}

		paths := make([]string, 0)
		err = protonDrive.ListDirectoriesRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, false, -1, 0, false, "", &paths)
		if err != nil {
			t.Fatal(err)
		}

		if len(paths) != 1 {
			t.Fatalf("Total path returned is differs from expected: %#v", paths)
		}
		if paths[0] != "/root" {
			t.Fatalf("Wrong root folder")
		}
	}
}

func TestCreateAndMoveAndDeleteFolderAtRoot(t *testing.T) {
	ctx, cancel, protonDrive := setup(t)
	t.Cleanup(func() {
		defer cancel()
		defer tearDown(t, ctx, protonDrive)
	})

	{
		/* Create folder src */
		_, err := protonDrive.CreateNewFolderByID(ctx, protonDrive.RootLink.LinkID, "src")
		if err != nil {
			t.Fatal(err)
		}

		paths := make([]string, 0)
		err = protonDrive.ListDirectoriesRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, false, -1, 0, true, "", &paths)
		if err != nil {
			t.Fatal(err)
		}

		if len(paths) != 1 {
			t.Fatalf("Total path returned is differs from expected: %#v", paths)
		}
		if paths[0] != "/src" {
			t.Fatalf("Wrong folder created")
		}

		paths = make([]string, 0)
		err = protonDrive.ListDirectoriesRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, false, -1, 0, false, "", &paths)
		if err != nil {
			t.Fatal(err)
		}

		if len(paths) != 2 {
			t.Fatalf("Total path returned is differs from expected: %#v", paths)
		}
		if paths[0] != "/root" {
			t.Fatalf("Wrong root folder")
		}
		if paths[1] != "/root/src" {
			t.Fatalf("Wrong folder created")
		}
	}

	{
		/* Create folder dest */
		_, err := protonDrive.CreateNewFolderByID(ctx, protonDrive.RootLink.LinkID, "dest")
		if err != nil {
			t.Fatal(err)
		}

		paths := make([]string, 0)
		err = protonDrive.ListDirectoriesRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, false, -1, 0, true, "", &paths)
		if err != nil {
			t.Fatal(err)
		}

		if len(paths) != 2 {
			t.Fatalf("Total path returned is differs from expected: %#v", paths)
		}
		if paths[0] != "/src" {
			t.Fatalf("Wrong folder created")
		}
		if paths[1] != "/dest" {
			t.Fatalf("Wrong folder created")
		}

		paths = make([]string, 0)
		err = protonDrive.ListDirectoriesRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, false, -1, 0, false, "", &paths)
		if err != nil {
			t.Fatal(err)
		}

		if len(paths) != 3 {
			t.Fatalf("Total path returned is differs from expected: %#v", paths)
		}
		if paths[0] != "/root" {
			t.Fatalf("Wrong root folder")
		}
		if paths[1] != "/root/src" {
			t.Fatalf("Wrong folder created")
		}
		if paths[2] != "/root/dest" {
			t.Fatalf("Wrong folder created")
		}
	}

	{
		/* Move folder src to under dest */
		targetSrcFolderLink, err := protonDrive.SearchByNameRecursivelyFromRoot(ctx, "src", true)
		if err != nil {
			t.Fatal(err)
		}
		targetDestFolderLink, err := protonDrive.SearchByNameRecursivelyFromRoot(ctx, "dest", true)
		if err != nil {
			t.Fatal(err)
		}
		if targetSrcFolderLink == nil || targetDestFolderLink == nil {
			t.Fatalf("Folder src/dest not found")
		} else {
			err := protonDrive.MoveFolder(ctx, targetSrcFolderLink, targetDestFolderLink, "src")
			if err != nil {
				t.Fatal(err)
			}
		}

		paths := make([]string, 0)
		err = protonDrive.ListDirectoriesRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, false, -1, 0, true, "", &paths)
		if err != nil {
			t.Fatal(err)
		}

		if len(paths) != 2 {
			t.Fatalf("Total path returned is differs from expected: %#v", paths)
		}
		if paths[0] != "/dest" {
			t.Fatalf("Wrong folder moved")
		}
		if paths[1] != "/dest/src" {
			t.Fatalf("Wrong folder moved")
		}

		paths = make([]string, 0)
		err = protonDrive.ListDirectoriesRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, false, -1, 0, false, "", &paths)
		if err != nil {
			t.Fatal(err)
		}

		if len(paths) != 3 {
			t.Fatalf("Total path returned is differs from expected: %#v", paths)
		}
		if paths[0] != "/root" {
			t.Fatalf("Wrong root folder")
		}
		if paths[1] != "/root/dest" {
			t.Fatalf("Wrong folder moved")
		}
		if paths[2] != "/root/dest/src" {
			t.Fatalf("Wrong folder moved")
		}
	}

	{
		/* Delete folder dest */
		targetFolderLink, err := protonDrive.SearchByNameRecursivelyFromRoot(ctx, "dest", true)
		if err != nil {
			t.Fatal(err)
		}
		if targetFolderLink == nil {
			t.Fatalf("Folder dest not found")
		} else {
			err = protonDrive.MoveFolderToTrashByID(ctx, targetFolderLink.LinkID, false)
			if err != nil {
				t.Fatal(err)
			}
		}

		paths := make([]string, 0)
		err = protonDrive.ListDirectoriesRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, false, -1, 0, false, "", &paths)
		if err != nil {
			t.Fatal(err)
		}

		if len(paths) != 1 {
			t.Fatalf("Total path returned is differs from expected: %#v", paths)
		}
		if paths[0] != "/root" {
			t.Fatalf("Wrong root folder")
		}
	}
}

func TestCreateAndMoveAndDeleteFolderAtRootWithFile(t *testing.T) {
	ctx, cancel, protonDrive := setup(t)
	t.Cleanup(func() {
		defer cancel()
		defer tearDown(t, ctx, protonDrive)
	})

	{
		/* Create folder src */
		_, err := protonDrive.CreateNewFolderByID(ctx, protonDrive.RootLink.LinkID, "src")
		if err != nil {
			t.Fatal(err)
		}

		paths := make([]string, 0)
		err = protonDrive.ListDirectoriesRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, false, -1, 0, true, "", &paths)
		if err != nil {
			t.Fatal(err)
		}

		if len(paths) != 1 {
			t.Fatalf("Total path returned is differs from expected: %#v", paths)
		}
		if paths[0] != "/src" {
			t.Fatalf("Wrong folder created")
		}

		paths = make([]string, 0)
		err = protonDrive.ListDirectoriesRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, false, -1, 0, false, "", &paths)
		if err != nil {
			t.Fatal(err)
		}

		if len(paths) != 2 {
			t.Fatalf("Total path returned is differs from expected: %#v", paths)
		}
		if paths[0] != "/root" {
			t.Fatalf("Wrong root folder")
		}
		if paths[1] != "/root/src" {
			t.Fatalf("Wrong folder created")
		}
	}

	{
		/* Upload a file integrationTestImage.png to src */
		targetSrcFolderLink, err := protonDrive.SearchByNameRecursivelyFromRoot(ctx, "src", true)
		if err != nil {
			t.Fatal(err)
		}

		if targetSrcFolderLink == nil {
			t.Fatalf("src folder should exist")
		} else {
			f, err := os.Open("testcase/integrationTestImage.png")
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()

			info, err := os.Stat("testcase/integrationTestImage.png")
			if err != nil {
				t.Fatal(err)
			}

			in := bufio.NewReader(f)

			_, _, err = protonDrive.UploadFileByReader(ctx, targetSrcFolderLink.LinkID, "integrationTestImage.png", info.ModTime(), in)
			if err != nil {
				t.Fatal(err)
			}
		}

		paths := make([]string, 0)
		err = protonDrive.ListDirectoriesRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, false, -1, 0, true, "", &paths)
		if err != nil {
			t.Fatal(err)
		}

		if len(paths) != 2 {
			t.Fatalf("Total path returned is not as expected: %#v", paths)
		}
		if paths[0] != "/src" {
			t.Fatalf("Wrong folder name")
		}
		if paths[1] != "/src/integrationTestImage.png" {
			t.Fatalf("Wrong file name decrypted")
		}

		paths = make([]string, 0)
		err = protonDrive.ListDirectoriesRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, false, -1, 0, false, "", &paths)
		if err != nil {
			t.Fatal(err)
		}

		if len(paths) != 3 {
			t.Fatalf("Total path returned is differs from expected: %#v", paths)
		}
		if paths[0] != "/root" {
			t.Fatalf("Wrong root folder")
		}
		if paths[1] != "/root/src" {
			t.Fatalf("Wrong folder name")
		}
		if paths[2] != "/root/src/integrationTestImage.png" {
			t.Fatalf("Wrong file name decrypted")
		}
	}

	{
		/* Create folder dest */
		_, err := protonDrive.CreateNewFolderByID(ctx, protonDrive.RootLink.LinkID, "dest")
		if err != nil {
			t.Fatal(err)
		}

		paths := make([]string, 0)
		err = protonDrive.ListDirectoriesRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, false, -1, 0, true, "", &paths)
		if err != nil {
			t.Fatal(err)
		}

		if len(paths) != 3 {
			t.Fatalf("Total path returned is differs from expected: %#v", paths)
		}
		if paths[0] != "/src" {
			t.Fatalf("Wrong folder created")
		}
		if paths[1] != "/src/integrationTestImage.png" {
			t.Fatalf("Wrong file created")
		}
		if paths[2] != "/dest" {
			t.Fatalf("Wrong folder created")
		}

		paths = make([]string, 0)
		err = protonDrive.ListDirectoriesRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, false, -1, 0, false, "", &paths)
		if err != nil {
			t.Fatal(err)
		}

		if len(paths) != 4 {
			t.Fatalf("Total path returned is differs from expected: %#v", paths)
		}
		if paths[0] != "/root" {
			t.Fatalf("Wrong root folder")
		}
		if paths[1] != "/root/src" {
			t.Fatalf("Wrong folder created")
		}
		if paths[2] != "/root/src/integrationTestImage.png" {
			t.Fatalf("Wrong file created")
		}
		if paths[3] != "/root/dest" {
			t.Fatalf("Wrong folder created")
		}
	}

	{
		/* Move folder src to under dest */
		targetSrcFolderLink, err := protonDrive.SearchByNameRecursivelyFromRoot(ctx, "src", true)
		if err != nil {
			t.Fatal(err)
		}
		targetDestFolderLink, err := protonDrive.SearchByNameRecursivelyFromRoot(ctx, "dest", true)
		if err != nil {
			t.Fatal(err)
		}
		if targetSrcFolderLink == nil || targetDestFolderLink == nil {
			t.Fatalf("Folder src/dest not found")
		} else {
			err := protonDrive.MoveFolder(ctx, targetSrcFolderLink, targetDestFolderLink, "newSrc")
			if err != nil {
				t.Fatal(err)
			}
		}

		targetSrcFolderLink, err = protonDrive.SearchByNameRecursivelyFromRoot(ctx, "newSrc", true)
		if err != nil {
			t.Fatal(err)
		}
		targetDestFolderLink, err = protonDrive.SearchByNameRecursivelyFromRoot(ctx, "dest", true)
		if err != nil {
			t.Fatal(err)
		}
		if targetSrcFolderLink == nil || targetDestFolderLink == nil {
			t.Fatalf("Folder newSrc/dest not found")
		}

		paths := make([]string, 0)
		err = protonDrive.ListDirectoriesRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, false, -1, 0, true, "", &paths)
		if err != nil {
			// if the move is done wrongly, the files within the folder won't be able to be decrypted
			// gopenpgp: error in reading message: openpgp: invalid data: parsing error
			t.Fatal(err)
		}

		if len(paths) != 3 {
			t.Fatalf("Total path returned is differs from expected: %#v", paths)
		}
		if paths[0] != "/dest" {
			t.Fatalf("Wrong folder moved")
		}
		if paths[1] != "/dest/newSrc" {
			t.Fatalf("Wrong folder moved")
		}
		if paths[2] != "/dest/newSrc/integrationTestImage.png" {
			t.Fatalf("Wrong file moved")
		}

		paths = make([]string, 0)
		err = protonDrive.ListDirectoriesRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, false, -1, 0, false, "", &paths)
		if err != nil {
			t.Fatal(err)
		}

		if len(paths) != 4 {
			t.Fatalf("Total path returned is differs from expected: %#v", paths)
		}
		if paths[0] != "/root" {
			t.Fatalf("Wrong root folder")
		}
		if paths[1] != "/root/dest" {
			t.Fatalf("Wrong folder moved")
		}
		if paths[2] != "/root/dest/newSrc" {
			t.Fatalf("Wrong folder moved")
		}
		if paths[3] != "/root/dest/newSrc/integrationTestImage.png" {
			t.Fatalf("Wrong file moved")
		}
	}

	{
		/* Delete folder dest */
		targetFolderLink, err := protonDrive.SearchByNameRecursivelyFromRoot(ctx, "dest", true)
		if err != nil {
			t.Fatal(err)
		}
		if targetFolderLink == nil {
			t.Fatalf("Folder dest not found")
		} else {
			err = protonDrive.MoveFolderToTrashByID(ctx, targetFolderLink.LinkID, false)
			if err != nil {
				t.Fatal(err)
			}
		}

		paths := make([]string, 0)
		err = protonDrive.ListDirectoriesRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, false, -1, 0, false, "", &paths)
		if err != nil {
			t.Fatal(err)
		}

		if len(paths) != 1 {
			t.Fatalf("Total path returned is differs from expected: %#v", paths)
		}
		if paths[0] != "/root" {
			t.Fatalf("Wrong root folder")
		}
	}
}
