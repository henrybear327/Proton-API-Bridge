package proton_api_bridge

import (
	"log"
	"strings"
	"testing"

	"github.com/henrybear327/go-proton-api"
)

func TestCreateAndDeleteFolder(t *testing.T) {
	ctx, cancel, protonDrive := setup(t, false)
	t.Cleanup(func() {
		defer cancel()
		defer tearDown(t, ctx, protonDrive)
	})

	log.Println("Create a folder tmp at root")
	createFolder(t, ctx, protonDrive, "", "tmp")
	checkActiveFileListing(t, ctx, protonDrive, []string{"/tmp"})

	log.Println("Delete folder tmp")
	deleteBySearchingFromRoot(t, ctx, protonDrive, "tmp", true, false)
	checkActiveFileListing(t, ctx, protonDrive, []string{})
}

func TestCreateAndCreateAndDeleteFolder(t *testing.T) {
	ctx, cancel, protonDrive := setup(t, false)
	t.Cleanup(func() {
		defer cancel()
		defer tearDown(t, ctx, protonDrive)
	})

	log.Println("Create a folder tmp at root")
	createFolder(t, ctx, protonDrive, "", "tmp")
	checkActiveFileListing(t, ctx, protonDrive, []string{"/tmp"})

	log.Println("Create a folder tmp at root again")
	createFolderExpectError(t, ctx, protonDrive, "", "tmp", proton.ErrFolderNameExist)
	checkActiveFileListing(t, ctx, protonDrive, []string{"/tmp"})

	log.Println("Delete folder tmp")
	deleteBySearchingFromRoot(t, ctx, protonDrive, "tmp", true, false)
}

func TestUploadAndDownloadAndDeleteAFile(t *testing.T) {
	ctx, cancel, protonDrive := setup(t, true)
	t.Cleanup(func() {
		defer cancel()
		defer tearDown(t, ctx, protonDrive)
	})

	log.Println("Upload integrationTestImage.png")
	uploadFileByFilepath(t, ctx, protonDrive, "", "integrationTestImage.png", "testcase/integrationTestImage.png", false)
	checkRevisions(protonDrive, ctx, t, "integrationTestImage.png", 1, 1, 0, 0)
	checkActiveFileListing(t, ctx, protonDrive, []string{"/integrationTestImage.png"})
	downloadFile(t, ctx, protonDrive, "", "integrationTestImage.png", "testcase/integrationTestImage.png", "")

	log.Println("Upload a new revision to replace integrationTestImage.png")
	uploadFileByFilepath(t, ctx, protonDrive, "", "integrationTestImage.png", "testcase/integrationTestImage2.png", false) /* Add a revision */
	checkRevisions(protonDrive, ctx, t, "integrationTestImage.png", 2, 1, 0, 1)
	downloadFile(t, ctx, protonDrive, "", "integrationTestImage.png", "testcase/integrationTestImage2.png", "")

	log.Println("Delete file integrationTestImage.png")
	deleteBySearchingFromRoot(t, ctx, protonDrive, "integrationTestImage.png", false, false)
	checkActiveFileListing(t, ctx, protonDrive, []string{})
}

func TestPartialUploadAndReuploadFailedAndDownloadAndDeleteAFile(t *testing.T) {
	ctx, cancel, protonDrive := setup(t, false)
	t.Cleanup(func() {
		defer cancel()
		defer tearDown(t, ctx, protonDrive)
	})

	log.Println("Partial upload a new draft revision of integrationTestImage.png")
	uploadFileByFilepath(t, ctx, protonDrive, "", "integrationTestImage.png", "testcase/integrationTestImage.png", true)
	checkRevisions(protonDrive, ctx, t, "integrationTestImage.png", 1, 0, 1, 0)
	checkActiveFileListing(t, ctx, protonDrive, []string{})

	log.Println("Partial upload a new draft revision of integrationTestImage.png again")
	uploadFileByFilepathWithError(t, ctx, protonDrive, "", "integrationTestImage.png", "testcase/integrationTestImage.png", true, ErrDraftExists)
	checkRevisions(protonDrive, ctx, t, "integrationTestImage.png", 1, 0, 1, 0)
	checkActiveFileListing(t, ctx, protonDrive, []string{})

	// FIXME: delete file with draft revision only
	// log.Println("Delete file integrationTestImage.png")
	// deleteBySearchingFromRoot(t, ctx, protonDrive, "integrationTestImage.png", false, true)
	// checkActiveFileListing(t, ctx, protonDrive, []string{})
}

func TestPartialUploadAndReuploadAndDownloadAndDeleteAFile(t *testing.T) {
	ctx, cancel, protonDrive := setup(t, true)
	t.Cleanup(func() {
		defer cancel()
		defer tearDown(t, ctx, protonDrive)
	})

	log.Println("Partial upload a new draft revision of integrationTestImage.png")
	uploadFileByFilepath(t, ctx, protonDrive, "", "integrationTestImage.png", "testcase/integrationTestImage.png", true)
	checkRevisions(protonDrive, ctx, t, "integrationTestImage.png", 1, 0, 1, 0)
	checkActiveFileListing(t, ctx, protonDrive, []string{})

	log.Println("Partial upload a new draft revision of integrationTestImage.png again")
	uploadFileByFilepath(t, ctx, protonDrive, "", "integrationTestImage.png", "testcase/integrationTestImage.png", true)
	checkRevisions(protonDrive, ctx, t, "integrationTestImage.png", 1, 0, 1, 0)
	checkActiveFileListing(t, ctx, protonDrive, []string{})

	log.Println("Upload a new revision and activates it to replace integrationTestImage.png")
	uploadFileByFilepath(t, ctx, protonDrive, "", "integrationTestImage.png", "testcase/integrationTestImage2.png", false) /* Add a revision */
	checkRevisions(protonDrive, ctx, t, "integrationTestImage.png", 1, 1, 0, 0)
	downloadFile(t, ctx, protonDrive, "", "integrationTestImage.png", "testcase/integrationTestImage2.png", "")
	checkActiveFileListing(t, ctx, protonDrive, []string{"/integrationTestImage.png"})

	log.Println("Delete file integrationTestImage.png")
	deleteBySearchingFromRoot(t, ctx, protonDrive, "integrationTestImage.png", false, false)
	checkActiveFileListing(t, ctx, protonDrive, []string{})
}

func TestUploadAndDeleteAnEmptyFileAtRoot(t *testing.T) {
	ctx, cancel, protonDrive := setup(t, false)
	t.Cleanup(func() {
		defer cancel()
		defer tearDown(t, ctx, protonDrive)
	})

	log.Println("Upload empty.txt")
	uploadFileByFilepath(t, ctx, protonDrive, "", "empty.txt", "testcase/empty.txt", false)
	checkRevisions(protonDrive, ctx, t, "empty.txt", 1, 1, 0, 0)
	checkActiveFileListing(t, ctx, protonDrive, []string{"/empty.txt"})
	downloadFile(t, ctx, protonDrive, "", "empty.txt", "testcase/empty.txt", "")

	log.Println("Upload a new revision to replace empty.txt")
	uploadFileByFilepath(t, ctx, protonDrive, "", "empty.txt", "testcase/empty.txt", false) /* Add a revision */
	checkRevisions(protonDrive, ctx, t, "empty.txt", 2, 1, 0, 1)
	downloadFile(t, ctx, protonDrive, "", "empty.txt", "testcase/empty.txt", "")
	checkActiveFileListing(t, ctx, protonDrive, []string{"/empty.txt"})

	log.Println("Delete file empty.txt")
	deleteBySearchingFromRoot(t, ctx, protonDrive, "empty.txt", false, false)
	checkActiveFileListing(t, ctx, protonDrive, []string{})
}

func TestUploadAndDownloadAndDeleteAFileAtAFolderOneLevelFromRoot(t *testing.T) {
	ctx, cancel, protonDrive := setup(t, false)
	t.Cleanup(func() {
		defer cancel()
		defer tearDown(t, ctx, protonDrive)
	})

	log.Println("Create folder level1")
	createFolder(t, ctx, protonDrive, "", "level1")
	checkActiveFileListing(t, ctx, protonDrive, []string{"/level1"})

	log.Println("Upload integrationTestImage.png to level1")
	uploadFileByFilepath(t, ctx, protonDrive, "level1", "integrationTestImage.png", "testcase/integrationTestImage.png", false)
	checkRevisions(protonDrive, ctx, t, "integrationTestImage.png", 1, 1, 0, 0)
	checkActiveFileListing(t, ctx, protonDrive, []string{"/level1", "/level1/integrationTestImage.png"})
	downloadFile(t, ctx, protonDrive, "level1", "integrationTestImage.png", "testcase/integrationTestImage.png", "")

	log.Println("Upload a new revision to replace integrationTestImage.png in level1")
	uploadFileByFilepath(t, ctx, protonDrive, "level1", "integrationTestImage.png", "testcase/integrationTestImage2.png", false) /* Add a revision */
	checkRevisions(protonDrive, ctx, t, "integrationTestImage.png", 2, 1, 0, 1)
	downloadFile(t, ctx, protonDrive, "level1", "integrationTestImage.png", "testcase/integrationTestImage2.png", "")

	log.Println("Delete folder level1")
	deleteBySearchingFromRoot(t, ctx, protonDrive, "level1", true, false)
	checkActiveFileListing(t, ctx, protonDrive, []string{})
}

func TestCreateAndMoveAndDeleteFolder(t *testing.T) {
	ctx, cancel, protonDrive := setup(t, false)
	t.Cleanup(func() {
		defer cancel()
		defer tearDown(t, ctx, protonDrive)
	})

	log.Println("Create a folder src at root")
	createFolder(t, ctx, protonDrive, "", "src")
	checkActiveFileListing(t, ctx, protonDrive, []string{"/src"})

	log.Println("Create a folder dst at root")
	createFolder(t, ctx, protonDrive, "", "dst")
	checkActiveFileListing(t, ctx, protonDrive, []string{"/src", "/dst"})

	log.Println("Move folder src to under folder dst")
	moveFolder(t, ctx, protonDrive, "src", "dst")
	checkActiveFileListing(t, ctx, protonDrive, []string{"/dst", "/dst/src"})

	log.Println("Delete folder dst")
	deleteBySearchingFromRoot(t, ctx, protonDrive, "dst", true, false)
	checkActiveFileListing(t, ctx, protonDrive, []string{})
}

func TestCreateAndMoveAndDeleteFolderWithAFile(t *testing.T) {
	ctx, cancel, protonDrive := setup(t, false)
	t.Cleanup(func() {
		defer cancel()
		defer tearDown(t, ctx, protonDrive)
	})

	log.Println("Create a folder src at root")
	createFolder(t, ctx, protonDrive, "", "src")
	checkActiveFileListing(t, ctx, protonDrive, []string{"/src"})

	log.Println("Upload integrationTestImage.png to src")
	uploadFileByFilepath(t, ctx, protonDrive, "src", "integrationTestImage.png", "testcase/integrationTestImage.png", false)
	checkRevisions(protonDrive, ctx, t, "integrationTestImage.png", 1, 1, 0, 0)
	checkActiveFileListing(t, ctx, protonDrive, []string{"/src", "/src/integrationTestImage.png"})
	downloadFile(t, ctx, protonDrive, "src", "integrationTestImage.png", "testcase/integrationTestImage.png", "")

	log.Println("Create a folder dst at root")
	createFolder(t, ctx, protonDrive, "", "dst")
	checkActiveFileListing(t, ctx, protonDrive, []string{"/src", "/src/integrationTestImage.png", "/dst"})

	log.Println("Upload a new revision to replace integrationTestImage.png in src")
	uploadFileByFilepath(t, ctx, protonDrive, "src", "integrationTestImage.png", "testcase/integrationTestImage2.png", false) /* Add a revision */
	checkRevisions(protonDrive, ctx, t, "integrationTestImage.png", 2, 1, 0, 1)
	downloadFile(t, ctx, protonDrive, "src", "integrationTestImage.png", "testcase/integrationTestImage2.png", "")
	checkActiveFileListing(t, ctx, protonDrive, []string{"/src", "/src/integrationTestImage.png", "/dst"})

	log.Println("Move folder src to under folder dst")
	moveFolder(t, ctx, protonDrive, "src", "dst")
	checkActiveFileListing(t, ctx, protonDrive, []string{"/dst", "/dst/src", "/dst/src/integrationTestImage.png"})

	log.Println("Delete folder dst")
	deleteBySearchingFromRoot(t, ctx, protonDrive, "dst", true, false)
	checkActiveFileListing(t, ctx, protonDrive, []string{})
}

func TestCreateAndMoveAndDeleteAFileOneLevelFromRoot(t *testing.T) {
	ctx, cancel, protonDrive := setup(t, false)
	t.Cleanup(func() {
		defer cancel()
		defer tearDown(t, ctx, protonDrive)
	})

	log.Println("Create a folder src at root")
	createFolder(t, ctx, protonDrive, "", "src")
	checkActiveFileListing(t, ctx, protonDrive, []string{"/src"})

	log.Println("Upload integrationTestImage.png to src")
	uploadFileByFilepath(t, ctx, protonDrive, "src", "integrationTestImage.png", "testcase/integrationTestImage.png", false)
	checkRevisions(protonDrive, ctx, t, "integrationTestImage.png", 1, 1, 0, 0)
	checkActiveFileListing(t, ctx, protonDrive, []string{"/src", "/src/integrationTestImage.png"})
	downloadFile(t, ctx, protonDrive, "src", "integrationTestImage.png", "testcase/integrationTestImage.png", "")

	log.Println("Create a folder dst at root")
	createFolder(t, ctx, protonDrive, "", "dst")
	checkActiveFileListing(t, ctx, protonDrive, []string{"/src", "/src/integrationTestImage.png", "/dst"})

	log.Println("Upload a new revision to replace integrationTestImage.png in src")
	uploadFileByFilepath(t, ctx, protonDrive, "src", "integrationTestImage.png", "testcase/integrationTestImage2.png", false) /* Add a revision */
	checkRevisions(protonDrive, ctx, t, "integrationTestImage.png", 2, 1, 0, 1)
	downloadFile(t, ctx, protonDrive, "src", "integrationTestImage.png", "testcase/integrationTestImage2.png", "")
	checkActiveFileListing(t, ctx, protonDrive, []string{"/src", "/src/integrationTestImage.png", "/dst"})

	log.Println("Move file integrationTestImage.png to under folder dst")
	moveFile(t, ctx, protonDrive, "integrationTestImage.png", "dst")
	checkActiveFileListing(t, ctx, protonDrive, []string{"/src", "/dst", "/dst/integrationTestImage.png"})

	log.Println("Delete folder dst")
	deleteBySearchingFromRoot(t, ctx, protonDrive, "dst", true, false)
	checkActiveFileListing(t, ctx, protonDrive, []string{"/src"})

	log.Println("Delete folder src")
	deleteBySearchingFromRoot(t, ctx, protonDrive, "src", true, false)
	checkActiveFileListing(t, ctx, protonDrive, []string{})
}

func TestUploadLargeNumberOfBlocks(t *testing.T) {
	ctx, cancel, protonDrive := setup(t, false)
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
	uploadFileByReader(t, ctx, protonDrive, "", filename, file1ContentReader, false)
	checkRevisions(protonDrive, ctx, t, filename, 1, 1, 0, 0)
	checkActiveFileListing(t, ctx, protonDrive, []string{"/" + filename})
	downloadFile(t, ctx, protonDrive, "", filename, "", file1Content)

	log.Println("Upload a new revision to replace fileContent.txt")
	uploadFileByReader(t, ctx, protonDrive, "", filename, file2ContentReader, false)
	checkRevisions(protonDrive, ctx, t, filename, 2, 1, 0, 1)
	checkActiveFileListing(t, ctx, protonDrive, []string{"/" + filename})
	downloadFile(t, ctx, protonDrive, "", filename, "", file2Content)

	log.Println("Delete file fileContent.txt")
	deleteBySearchingFromRoot(t, ctx, protonDrive, filename, false, false)
	checkActiveFileListing(t, ctx, protonDrive, []string{})
}
