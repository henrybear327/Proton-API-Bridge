package proton_api_bridge

import (
	"context"
	"log"
	"strings"
	"testing"

	"github.com/henrybear327/Proton-API-Bridge/common"
	"github.com/henrybear327/Proton-API-Bridge/utility"
)

func setup(t *testing.T) (context.Context, context.CancelFunc, *ProtonDrive) {
	utility.SetupLog()

	config := common.NewConfigForIntegrationTests()

	{
		// pre-condition check
		if !config.DestructiveIntegrationTest {
			t.Fatalf("CAUTION: the integration test requires a clean proton drive")
		}
		if !config.EmptyTrashAfterIntegrationTest {
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
func TestCreateAndDeleteFolder(t *testing.T) {
	ctx, cancel, protonDrive := setup(t)
	t.Cleanup(func() {
		defer cancel()
		defer tearDown(t, ctx, protonDrive)
	})

	log.Println("Create a folder tmp at root")
	createFolder(t, ctx, protonDrive, "", "tmp")
	checkFileListing(t, ctx, protonDrive, []string{"/tmp"})

	log.Println("Delet folder tmp")
	deleteBySearchingFromRoot(t, ctx, protonDrive, "tmp", true)
	checkFileListing(t, ctx, protonDrive, []string{})
}

func TestUploadAndDownloadAndDeleteAFile(t *testing.T) {
	ctx, cancel, protonDrive := setup(t)
	t.Cleanup(func() {
		defer cancel()
		defer tearDown(t, ctx, protonDrive)
	})

	log.Println("Upload integrationTestImage.png")
	uploadFileByFilepath(t, ctx, protonDrive, "", "integrationTestImage.png", "testcase/integrationTestImage.png")
	checkRevisions(protonDrive, ctx, t, "integrationTestImage.png", 1)
	checkFileListing(t, ctx, protonDrive, []string{"/integrationTestImage.png"})
	downloadFile(t, ctx, protonDrive, "", "integrationTestImage.png", "testcase/integrationTestImage.png", "")

	log.Println("Upload a new revision to replace integrationTestImage.png")
	uploadFileByFilepath(t, ctx, protonDrive, "", "integrationTestImage.png", "testcase/integrationTestImage2.png") /* Add a revision */
	checkRevisions(protonDrive, ctx, t, "integrationTestImage.png", 2)
	downloadFile(t, ctx, protonDrive, "", "integrationTestImage.png", "testcase/integrationTestImage2.png", "")
	checkFileListing(t, ctx, protonDrive, []string{"/integrationTestImage.png"})

	log.Println("Delete file integrationTestImage.png")
	deleteBySearchingFromRoot(t, ctx, protonDrive, "integrationTestImage.png", false)
	checkFileListing(t, ctx, protonDrive, []string{})
}

func TestUploadAndDeleteAnEmptyFileAtRoot(t *testing.T) {
	ctx, cancel, protonDrive := setup(t)
	t.Cleanup(func() {
		defer cancel()
		defer tearDown(t, ctx, protonDrive)
	})

	log.Println("Upload empty.txt")
	uploadFileByFilepath(t, ctx, protonDrive, "", "empty.txt", "testcase/empty.txt")
	checkRevisions(protonDrive, ctx, t, "empty.txt", 1)
	checkFileListing(t, ctx, protonDrive, []string{"/empty.txt"})
	downloadFile(t, ctx, protonDrive, "", "empty.txt", "testcase/empty.txt", "")

	log.Println("Upload a new revision to replace empty.txt")
	uploadFileByFilepath(t, ctx, protonDrive, "", "empty.txt", "testcase/empty.txt") /* Add a revision */
	checkRevisions(protonDrive, ctx, t, "empty.txt", 2)
	downloadFile(t, ctx, protonDrive, "", "empty.txt", "testcase/empty.txt", "")
	checkFileListing(t, ctx, protonDrive, []string{"/empty.txt"})

	log.Println("Delete file empty.txt")
	deleteBySearchingFromRoot(t, ctx, protonDrive, "empty.txt", false)
	checkFileListing(t, ctx, protonDrive, []string{})
}

func TestUploadAndDownloadAndDeleteAFileAtAFolderOneLevelFromRoot(t *testing.T) {
	ctx, cancel, protonDrive := setup(t)
	t.Cleanup(func() {
		defer cancel()
		defer tearDown(t, ctx, protonDrive)
	})

	log.Println("Create folder level1")
	createFolder(t, ctx, protonDrive, "", "level1")
	checkFileListing(t, ctx, protonDrive, []string{"/level1"})

	log.Println("Upload integrationTestImage.png to level1")
	uploadFileByFilepath(t, ctx, protonDrive, "level1", "integrationTestImage.png", "testcase/integrationTestImage.png")
	checkRevisions(protonDrive, ctx, t, "integrationTestImage.png", 1)
	checkFileListing(t, ctx, protonDrive, []string{"/level1", "/level1/integrationTestImage.png"})
	downloadFile(t, ctx, protonDrive, "level1", "integrationTestImage.png", "testcase/integrationTestImage.png", "")

	log.Println("Upload a new revision to replace integrationTestImage.png in level1")
	uploadFileByFilepath(t, ctx, protonDrive, "level1", "integrationTestImage.png", "testcase/integrationTestImage2.png") /* Add a revision */
	checkRevisions(protonDrive, ctx, t, "integrationTestImage.png", 2)
	downloadFile(t, ctx, protonDrive, "level1", "integrationTestImage.png", "testcase/integrationTestImage2.png", "")

	log.Println("Delete folder level1")
	deleteBySearchingFromRoot(t, ctx, protonDrive, "level1", true)
	checkFileListing(t, ctx, protonDrive, []string{})
}

func TestCreateAndMoveAndDeleteFolder(t *testing.T) {
	ctx, cancel, protonDrive := setup(t)
	t.Cleanup(func() {
		defer cancel()
		defer tearDown(t, ctx, protonDrive)
	})

	log.Println("Create a folder src at root")
	createFolder(t, ctx, protonDrive, "", "src")
	checkFileListing(t, ctx, protonDrive, []string{"/src"})

	log.Println("Create a folder dst at root")
	createFolder(t, ctx, protonDrive, "", "dst")
	checkFileListing(t, ctx, protonDrive, []string{"/src", "/dst"})

	log.Println("Move folder src to under folder dst")
	moveFolder(t, ctx, protonDrive, "src", "dst")
	checkFileListing(t, ctx, protonDrive, []string{"/dst", "/dst/src"})

	log.Println("Delete folder dst")
	deleteBySearchingFromRoot(t, ctx, protonDrive, "dst", true)
	checkFileListing(t, ctx, protonDrive, []string{})
}

func TestCreateAndMoveAndDeleteFolderWithAFile(t *testing.T) {
	ctx, cancel, protonDrive := setup(t)
	t.Cleanup(func() {
		defer cancel()
		defer tearDown(t, ctx, protonDrive)
	})

	log.Println("Create a folder src at root")
	createFolder(t, ctx, protonDrive, "", "src")
	checkFileListing(t, ctx, protonDrive, []string{"/src"})

	log.Println("Upload integrationTestImage.png to src")
	uploadFileByFilepath(t, ctx, protonDrive, "src", "integrationTestImage.png", "testcase/integrationTestImage.png")
	checkRevisions(protonDrive, ctx, t, "integrationTestImage.png", 1)
	checkFileListing(t, ctx, protonDrive, []string{"/src", "/src/integrationTestImage.png"})
	downloadFile(t, ctx, protonDrive, "src", "integrationTestImage.png", "testcase/integrationTestImage.png", "")

	log.Println("Create a folder dst at root")
	createFolder(t, ctx, protonDrive, "", "dst")
	checkFileListing(t, ctx, protonDrive, []string{"/src", "/src/integrationTestImage.png", "/dst"})

	log.Println("Upload a new revision to replace integrationTestImage.png in src")
	uploadFileByFilepath(t, ctx, protonDrive, "src", "integrationTestImage.png", "testcase/integrationTestImage2.png") /* Add a revision */
	checkRevisions(protonDrive, ctx, t, "integrationTestImage.png", 2)
	downloadFile(t, ctx, protonDrive, "src", "integrationTestImage.png", "testcase/integrationTestImage2.png", "")
	checkFileListing(t, ctx, protonDrive, []string{"/src", "/src/integrationTestImage.png", "/dst"})

	log.Println("Move folder src to under folder dst")
	moveFolder(t, ctx, protonDrive, "src", "dst")
	checkFileListing(t, ctx, protonDrive, []string{"/dst", "/dst/src", "/dst/src/integrationTestImage.png"})

	log.Println("Delete folder dst")
	deleteBySearchingFromRoot(t, ctx, protonDrive, "dst", true)
	checkFileListing(t, ctx, protonDrive, []string{})
}

func TestCreateAndMoveAndDeleteAFileOneLevelFromRoot(t *testing.T) {
	ctx, cancel, protonDrive := setup(t)
	t.Cleanup(func() {
		defer cancel()
		defer tearDown(t, ctx, protonDrive)
	})

	log.Println("Create a folder src at root")
	createFolder(t, ctx, protonDrive, "", "src")
	checkFileListing(t, ctx, protonDrive, []string{"/src"})

	log.Println("Upload integrationTestImage.png to src")
	uploadFileByFilepath(t, ctx, protonDrive, "src", "integrationTestImage.png", "testcase/integrationTestImage.png")
	checkRevisions(protonDrive, ctx, t, "integrationTestImage.png", 1)
	checkFileListing(t, ctx, protonDrive, []string{"/src", "/src/integrationTestImage.png"})
	downloadFile(t, ctx, protonDrive, "src", "integrationTestImage.png", "testcase/integrationTestImage.png", "")

	log.Println("Create a folder dst at root")
	createFolder(t, ctx, protonDrive, "", "dst")
	checkFileListing(t, ctx, protonDrive, []string{"/src", "/src/integrationTestImage.png", "/dst"})

	log.Println("Upload a new revision to replace integrationTestImage.png in src")
	uploadFileByFilepath(t, ctx, protonDrive, "src", "integrationTestImage.png", "testcase/integrationTestImage2.png") /* Add a revision */
	checkRevisions(protonDrive, ctx, t, "integrationTestImage.png", 2)
	downloadFile(t, ctx, protonDrive, "src", "integrationTestImage.png", "testcase/integrationTestImage2.png", "")
	checkFileListing(t, ctx, protonDrive, []string{"/src", "/src/integrationTestImage.png", "/dst"})

	log.Println("Move file integrationTestImage.png to under folder dst")
	moveFile(t, ctx, protonDrive, "integrationTestImage.png", "dst")
	checkFileListing(t, ctx, protonDrive, []string{"/src", "/dst", "/dst/integrationTestImage.png"})

	log.Println("Delete folder dst")
	deleteBySearchingFromRoot(t, ctx, protonDrive, "dst", true)
	checkFileListing(t, ctx, protonDrive, []string{"/src"})

	log.Println("Delete folder src")
	deleteBySearchingFromRoot(t, ctx, protonDrive, "src", true)
	checkFileListing(t, ctx, protonDrive, []string{})
}

func TestUploadLargeNumberOfBlocks(t *testing.T) {
	ctx, cancel, protonDrive := setup(t)
	t.Cleanup(func() {
		defer cancel()
		defer tearDown(t, ctx, protonDrive)
	})

	// in order to simulate uploading large files
	// we use 1KB for the UPLOAD_BLOCK_SIZE
	// so a 1000KB file will generate 1000 blocks to test the uploading mechanism
	// and also testing the downloading mechanism
	ORIGINAL_UPLOAD_BLOCK_SIZE := UPLOAD_BLOCK_SIZE
	defer func() {
		UPLOAD_BLOCK_SIZE = ORIGINAL_UPLOAD_BLOCK_SIZE
	}()
	blocks := 100
	UPLOAD_BLOCK_SIZE = 10

	filename := "fileContent.txt"
	file1Content := RandomString(UPLOAD_BLOCK_SIZE * blocks)
	file1ContentReader := strings.NewReader(file1Content)
	file2Content := RandomString(UPLOAD_BLOCK_SIZE * blocks)
	file2ContentReader := strings.NewReader(file2Content)

	log.Println("Upload fileContent.txt")
	uploadFileByReader(t, ctx, protonDrive, "", filename, file1ContentReader)
	checkRevisions(protonDrive, ctx, t, filename, 1)
	checkFileListing(t, ctx, protonDrive, []string{"/" + filename})
	downloadFile(t, ctx, protonDrive, "", filename, "", file1Content)

	log.Println("Upload a new revision to replace fileContent.txt")
	uploadFileByReader(t, ctx, protonDrive, "", filename, file2ContentReader)
	checkRevisions(protonDrive, ctx, t, filename, 2)
	checkFileListing(t, ctx, protonDrive, []string{"/" + filename})
	downloadFile(t, ctx, protonDrive, "", filename, "", file2Content)

	log.Println("Delete file fileContent.txt")
	deleteBySearchingFromRoot(t, ctx, protonDrive, filename, false)
	checkFileListing(t, ctx, protonDrive, []string{})
}
