package proton_api_bridge

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"log"
	"mime"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/henrybear327/go-proton-api"
	"github.com/relvacode/iso8601"
)

type FileSystemAttrs struct {
	ModificationTime time.Time
	Size             int64
}

func (protonDrive *ProtonDrive) DownloadFileByID(ctx context.Context, linkID string) ([]byte, *FileSystemAttrs, error) {
	link, err := protonDrive.getLink(ctx, linkID)
	if err != nil {
		return nil, nil, err
	}

	return protonDrive.DownloadFile(ctx, link)
}

func (protonDrive *ProtonDrive) GetRevisions(ctx context.Context, link *proton.Link, revisionType proton.RevisionState) ([]*proton.RevisionMetadata, error) {
	revisions, err := protonDrive.c.ListRevisions(ctx, protonDrive.MainShare.ShareID, link.LinkID)
	if err != nil {
		return nil, err
	}

	ret := make([]*proton.RevisionMetadata, 0)
	// Revisions are only for files, they represent “versions” of files.
	// Each file can have 1 active/draft revision and n obsolete revisions.
	for i := range revisions {
		if revisions[i].State == revisionType {
			ret = append(ret, &revisions[i])
		}
	}

	return ret, nil
}

func (protonDrive *ProtonDrive) GetActiveRevisionWithAttrs(ctx context.Context, link *proton.Link) (*proton.Revision, *FileSystemAttrs, error) {
	if link == nil {
		return nil, nil, ErrLinkMustNotBeNil
	}

	revisionsMetadata, err := protonDrive.GetRevisions(ctx, link, proton.RevisionStateActive)
	if err != nil {
		return nil, nil, err
	}

	if len(revisionsMetadata) != 1 {
		return nil, nil, ErrCantFindActiveRevision
	}

	revision, err := protonDrive.c.GetRevisionAllBlocks(ctx, protonDrive.MainShare.ShareID, link.LinkID, revisionsMetadata[0].ID)
	if err != nil {
		return nil, nil, err
	}

	nodeKR, err := protonDrive.getNodeKR(ctx, link)
	if err != nil {
		return nil, nil, err
	}

	revisionXAttrCommon, err := revision.GetDecXAttrString(protonDrive.AddrKR, nodeKR)
	if err != nil {
		return nil, nil, err
	}

	modificationTime, err := iso8601.ParseString(revisionXAttrCommon.ModificationTime)
	if err != nil {
		return nil, nil, err
	}

	return &revision, &FileSystemAttrs{
		ModificationTime: modificationTime,
		Size:             revisionXAttrCommon.Size,
	}, nil
}

func (protonDrive *ProtonDrive) DownloadFile(ctx context.Context, link *proton.Link) ([]byte, *FileSystemAttrs, error) {
	if link.Type != proton.LinkTypeFile {
		return nil, nil, ErrLinkTypeMustToBeFileType
	}

	parentNodeKR, err := protonDrive.getNodeKRByID(ctx, link.ParentLinkID)
	if err != nil {
		return nil, nil, err
	}

	nodeKR, err := link.GetKeyRing(parentNodeKR, protonDrive.AddrKR)
	if err != nil {
		return nil, nil, err
	}

	sessionKey, err := link.GetSessionKey(protonDrive.AddrKR, nodeKR)
	if err != nil {
		return nil, nil, err
	}

	revision, fileSystemAttrs, err := protonDrive.GetActiveRevisionWithAttrs(ctx, link)
	if err != nil {
		return nil, nil, err
	}

	buffer := bytes.NewBuffer(nil)
	for i := range revision.Blocks {
		// TODO: parallel download
		blockReader, err := protonDrive.c.GetBlock(ctx, revision.Blocks[i].BareURL, revision.Blocks[i].Token)
		if err != nil {
			return nil, nil, err
		}
		defer blockReader.Close()

		err = decryptBlockIntoBuffer(sessionKey, protonDrive.AddrKR, nodeKR, revision.Blocks[i].Hash, revision.Blocks[i].EncSignature, buffer, blockReader)
		if err != nil {
			return nil, nil, err
		}
	}

	if fileSystemAttrs != nil {
		return buffer.Bytes(), fileSystemAttrs, nil
	}
	return buffer.Bytes(), nil, nil
}

func (protonDrive *ProtonDrive) UploadFileByReader(ctx context.Context, parentLinkID string, filename string, modTime time.Time, file io.Reader, testParam int) (*proton.Link, int64, error) {
	parentLink, err := protonDrive.getLink(ctx, parentLinkID)
	if err != nil {
		return nil, 0, err
	}

	return protonDrive.uploadFile(ctx, parentLink, filename, modTime, file, testParam)
}

func (protonDrive *ProtonDrive) UploadFileByPath(ctx context.Context, parentLink *proton.Link, filename string, filePath string, testParam int) (*proton.Link, int64, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, 0, err
	}

	in := bufio.NewReader(f)

	return protonDrive.uploadFile(ctx, parentLink, filename, info.ModTime(), in, testParam)
}

func (protonDrive *ProtonDrive) handleRevisionConflict(ctx context.Context, link *proton.Link, createFileResp *proton.CreateFileRes) (string, bool, error) {
	if link != nil {
		linkID := link.LinkID

		draftRevision, err := protonDrive.GetRevisions(ctx, link, proton.RevisionStateDraft)
		if err != nil {
			return "", false, err
		}

		// if we have a draft revision, depending on the user config, we can abort the upload or recreate a draft
		// if we have no draft revision, then we can create a new draft revision directly (there is a restriction of 1 draft revision per file)
		if len(draftRevision) > 0 {
			// TODO: maintain clientUID to mark that this is our own draft (which can indicate failed upload attempt!)
			if protonDrive.Config.ReplaceExistingDraft {
				// Question: how do we observe for file upload cancellation -> clientUID?
				// Random thoughts: if there are concurrent modification to the draft, the server should be able to catch this when commiting the revision
				// since the manifestSignature (hash) will fail to match

				// delete the draft revision (will fail if the file only have a draft but no active revisions)
				if link.State == proton.LinkStateDraft {
					// delete the link (skipping trash, otherwise it won't work)
					err = protonDrive.c.DeleteChildren(ctx, protonDrive.MainShare.ShareID, link.ParentLinkID, linkID)
					if err != nil {
						return "", false, err
					}

					return "", true, nil
				}

				// delete the draft revision
				err = protonDrive.c.DeleteRevision(ctx, protonDrive.MainShare.ShareID, linkID, draftRevision[0].ID)
				if err != nil {
					return "", false, err
				}
			} else {
				// if there is a draft, based on the web behavior, it will ask if the user wants to replace the failed upload attempt
				// current behavior, we report an error to not upload the file (conservative)
				return "", false, ErrDraftExists
			}
		}

		// create a new revision
		newRevision, err := protonDrive.c.CreateRevision(ctx, protonDrive.MainShare.ShareID, linkID)
		if err != nil {
			return "", false, err
		}

		return newRevision.ID, false, nil
	} else if createFileResp != nil {
		return createFileResp.RevisionID, false, nil
	} else {
		// should not happen anymore, since the file search will include the draft now
		return "", false, ErrInternalErrorOnFileUpload
	}
}

func (protonDrive *ProtonDrive) createFileUploadDraft(ctx context.Context, parentLink *proton.Link, filename string, modTime time.Time, mimeType string) (string, string, *crypto.SessionKey, *crypto.KeyRing, error) {
	parentNodeKR, err := protonDrive.getNodeKR(ctx, parentLink)
	if err != nil {
		return "", "", nil, nil, err
	}

	newNodeKey, newNodePassphraseEnc, newNodePassphraseSignature, err := generateNodeKeys(parentNodeKR, protonDrive.AddrKR)
	if err != nil {
		return "", "", nil, nil, err
	}

	createFileReq := proton.CreateFileReq{
		ParentLinkID: parentLink.LinkID,

		// Name     string // Encrypted File Name
		// Hash     string // Encrypted File Name hash
		MIMEType: mimeType, // MIME Type

		// ContentKeyPacket          string // The block's key packet, encrypted with the node key.
		// ContentKeyPacketSignature string // Unencrypted signature of the content session key, signed with the NodeKey

		NodeKey:                 newNodeKey,                 // The private NodeKey, used to decrypt any file/folder content.
		NodePassphrase:          newNodePassphraseEnc,       // The passphrase used to unlock the NodeKey, encrypted by the owning Link/Share keyring.
		NodePassphraseSignature: newNodePassphraseSignature, // The signature of the NodePassphrase

		SignatureAddress: protonDrive.signatureAddress, // Signature email address used to sign passphrase and name
	}

	/* Name is encrypted using the parent's keyring, and signed with address key */
	err = createFileReq.SetName(filename, protonDrive.AddrKR, parentNodeKR)
	if err != nil {
		return "", "", nil, nil, err
	}

	parentHashKey, err := parentLink.GetHashKey(parentNodeKR)
	if err != nil {
		return "", "", nil, nil, err
	}

	newNodeKR, err := getKeyRing(parentNodeKR, protonDrive.AddrKR, newNodeKey, newNodePassphraseEnc, newNodePassphraseSignature)
	if err != nil {
		return "", "", nil, nil, err
	}

	err = createFileReq.SetHash(filename, parentHashKey)
	if err != nil {
		return "", "", nil, nil, err
	}

	newSessionKey, err := createFileReq.SetContentKeyPacketAndSignature(newNodeKR, protonDrive.AddrKR)
	if err != nil {
		return "", "", nil, nil, err
	}

	createFileAction := func() (*proton.CreateFileRes, *proton.Link, error) {
		createFileResp, err := protonDrive.c.CreateFile(ctx, protonDrive.MainShare.ShareID, createFileReq)
		if err != nil {
			// FIXME: check for duplicated filename by relying on checkAvailableHashes
			// Also saving generating resources such as new nodeKR, etc.

			if err != proton.ErrFileNameExist {
				// other real error caught
				return nil, nil, err
			}

			// search for the link within this folder which has an active/draft revision as we have a file creation conflict
			link, err := protonDrive.SearchByNameInActiveFolder(ctx, parentLink, filename, true, false, proton.LinkStateActive)
			if err != nil {
				return nil, nil, err
			}

			if link == nil {
				link, err = protonDrive.SearchByNameInActiveFolder(ctx, parentLink, filename, true, false, proton.LinkStateDraft)
				if err != nil {
					return nil, nil, err
				}

				if link == nil {
					// we have a real problem here (unless the assumption is wrong)
					// since we can't create a new file AND we can't locate a file with active/draft revision in it
					return nil, nil, ErrCantLocateRevision
				}
			}

			return nil, link, nil
		}

		return &createFileResp, nil, nil
	}

	createFileResp, link, err := createFileAction()
	if err != nil {
		return "", "", nil, nil, err
	}

	revisionID, shouldSubmitCreateFileRequestAgain, err := protonDrive.handleRevisionConflict(ctx, link, createFileResp)
	if err != nil {
		return "", "", nil, nil, err
	}

	if shouldSubmitCreateFileRequestAgain {
		// the case where the link has only a draft but no active revision
		// we need to delete the link and recreate one
		createFileResp, link, err = createFileAction()
		if err != nil {
			return "", "", nil, nil, err
		}

		revisionID, _, err = protonDrive.handleRevisionConflict(ctx, link, createFileResp)
		if err != nil {
			return "", "", nil, nil, err
		}
	}

	linkID := ""
	if link != nil {
		linkID = link.LinkID

		// get original newSessionKey and newNodeKR
		parentNodeKR, err = protonDrive.getNodeKRByID(ctx, link.ParentLinkID)
		if err != nil {
			return "", "", nil, nil, err
		}
		newNodeKR, err = link.GetKeyRing(parentNodeKR, protonDrive.AddrKR)
		if err != nil {
			return "", "", nil, nil, err
		}
		newSessionKey, err = link.GetSessionKey(protonDrive.AddrKR, newNodeKR)
		if err != nil {
			return "", "", nil, nil, err
		}
	} else {
		linkID = createFileResp.ID
	}

	return linkID, revisionID, newSessionKey, newNodeKR, nil
}

func (protonDrive *ProtonDrive) uploadAndCollectBlockData(ctx context.Context, newSessionKey *crypto.SessionKey, newNodeKR *crypto.KeyRing, file io.Reader, linkID, revisionID string) ([]byte, int64, error) {
	type PendingUploadBlocks struct {
		blockUploadInfo proton.BlockUploadInfo
		encData         []byte
	}

	if newSessionKey == nil || newNodeKR == nil {
		return nil, 0, ErrMissingInputUploadAndCollectBlockData
	}

	totalFileSize := int64(0)

	pendingUploadBlocks := make([]PendingUploadBlocks, 0)
	manifestSignatureData := make([]byte, 0)
	uploadPendingBlocks := func() error {
		if len(pendingUploadBlocks) == 0 {
			return nil
		}

		blockList := make([]proton.BlockUploadInfo, 0)
		for i := range pendingUploadBlocks {
			blockList = append(blockList, pendingUploadBlocks[i].blockUploadInfo)
		}
		blockUploadReq := proton.BlockUploadReq{
			AddressID:  protonDrive.MainShare.AddressID,
			ShareID:    protonDrive.MainShare.ShareID,
			LinkID:     linkID,
			RevisionID: revisionID,

			BlockList: blockList,
		}
		blockUploadResp, err := protonDrive.c.RequestBlockUpload(ctx, blockUploadReq)
		if err != nil {
			return err
		}

		for i := range blockUploadResp {
			err := protonDrive.c.UploadBlock(ctx, blockUploadResp[i].BareURL, blockUploadResp[i].Token, bytes.NewReader(pendingUploadBlocks[i].encData))
			if err != nil {
				return err
			}
		}

		pendingUploadBlocks = pendingUploadBlocks[:0]

		return nil
	}

	shouldContinue := true
	processData := func(blockIndex int, data []byte, wg *sync.WaitGroup, errorCh chan error) {
		defer protonDrive.uploadEncryptionSem.Release(1)
		defer wg.Done()

		log.Println("processData start", linkID, blockIndex)
		defer log.Println("processData done", linkID, blockIndex)

		// encrypt data
		dataPlainMessage := crypto.NewPlainMessage(data)
		encData, err := newSessionKey.Encrypt(dataPlainMessage)
		if err != nil {
			errorCh <- err
			return
		}

		encSignature, err := protonDrive.AddrKR.SignDetachedEncrypted(dataPlainMessage, newNodeKR)
		if err != nil {
			errorCh <- err
			return
		}
		encSignatureStr, err := encSignature.GetArmored()
		if err != nil {
			errorCh <- err
			return
		}

		h := sha256.New()
		h.Write(encData)
		hash := h.Sum(nil)
		base64Hash := base64.StdEncoding.EncodeToString(hash)
		if err != nil {
			errorCh <- err
			return
		}
		manifestSignatureData = append(manifestSignatureData, hash...)

		pendingUploadBlocks = append(pendingUploadBlocks, PendingUploadBlocks{
			blockUploadInfo: proton.BlockUploadInfo{
				Index:        blockIndex, // iOS drive: BE starts with 1
				Size:         int64(len(encData)),
				EncSignature: encSignatureStr,
				Hash:         base64Hash,
			},
			encData: encData,
		})

		errorCh <- nil
	}
	var wg sync.WaitGroup
	errorCh := make(chan error, UPLOAD_BATCH_BLOCK_SIZE)
	for i := 1; shouldContinue; i++ {
		log.Println(linkID, i)
		// read at most data of size UPLOAD_BLOCK_SIZE
		data := make([]byte, UPLOAD_BLOCK_SIZE) // FIXME: get block size from the server config instead of hardcoding it
		readBytes, err := file.Read(data)

		if err != nil {
			if err == io.EOF {
				// might still have data to read!
				if readBytes == 0 {
					break
				}
				shouldContinue = false
			} else {
				// all other errors
				return nil, 0, err
			}
		}
		data = data[:readBytes]
		totalFileSize += int64(readBytes)

		if err := protonDrive.uploadEncryptionSem.Acquire(ctx, 1); err != nil {
			return nil, 0, err
		}
		wg.Add(1)
		go processData(i, data, &wg, errorCh)
		if err != nil {
			return nil, 0, err
		}

		if (i-1) > 0 && (i-1)%UPLOAD_BATCH_BLOCK_SIZE == 0 {
			wg.Wait()
			close(errorCh)
			for err := range errorCh {
				if err != nil {
					return nil, 0, err
				}
			}
			err = uploadPendingBlocks()
			if err != nil {
				return nil, 0, err
			}

			errorCh = make(chan error, UPLOAD_BATCH_BLOCK_SIZE)
		}
	}
	wg.Wait()
	close(errorCh)
	for err := range errorCh {
		if err != nil {
			return nil, 0, err
		}
	}
	err := uploadPendingBlocks()
	if err != nil {
		return nil, 0, err
	}

	return manifestSignatureData, totalFileSize, nil
}

func (protonDrive *ProtonDrive) commitNewRevision(ctx context.Context, nodeKR *crypto.KeyRing, modificationTime time.Time, size int64, manifestSignatureData []byte, linkID, revisionID string) error {
	manifestSignature, err := protonDrive.AddrKR.SignDetached(crypto.NewPlainMessage(manifestSignatureData))
	if err != nil {
		return err
	}
	manifestSignatureString, err := manifestSignature.GetArmored()
	if err != nil {
		return err
	}

	commitRevisionReq := proton.CommitRevisionReq{
		ManifestSignature: manifestSignatureString,
		SignatureAddress:  protonDrive.signatureAddress,
	}
	err = commitRevisionReq.SetEncXAttrString(protonDrive.AddrKR, nodeKR, modificationTime, size)
	if err != nil {
		return err
	}

	err = protonDrive.c.CommitRevision(ctx, protonDrive.MainShare.ShareID, linkID, revisionID, commitRevisionReq)
	if err != nil {
		return err
	}

	return nil
}

// testParam is for integration test only
// 0 = normal mode
// 1 = up to create revision
// 2 = up to block upload
func (protonDrive *ProtonDrive) uploadFile(ctx context.Context, parentLink *proton.Link, filename string, modTime time.Time, file io.Reader, testParam int) (*proton.Link, int64, error) {
	// TODO: if we should use github.com/gabriel-vasile/mimetype to detect the MIME type from the file content itself
	// Note: this approach might cause the upload progress to display the "fake" progress, since we read in all the content all-at-once
	// mimetype.SetLimit(0)
	// mType := mimetype.Detect(fileContent)
	// mimeType := mType.String()

	// detect MIME type by looking at the filename only
	mimeType := mime.TypeByExtension(filepath.Ext(filename))
	if mimeType == "" {
		// api requires a mime type passed in
		mimeType = "text/plain"
	}

	/* step 1: create a draft */
	linkID, revisionID, newSessionKey, newNodeKR, err := protonDrive.createFileUploadDraft(ctx, parentLink, filename, modTime, mimeType)
	if err != nil {
		return nil, 0, err
	}

	if testParam == 1 {
		// for integration tests
		// we try to simulate only draft is created but no upload is performed yet
		finalLink, err := protonDrive.getLink(ctx, linkID)
		if err != nil {
			return nil, 0, err
		}
		return finalLink, 0, nil
	}

	/* step 2: upload blocks and collect block data */
	manifestSignature, fileSize, err := protonDrive.uploadAndCollectBlockData(ctx, newSessionKey, newNodeKR, file, linkID, revisionID)
	if err != nil {
		return nil, 0, err
	}

	if testParam == 2 {
		// for integration tests
		// we try to simulate blocks uploaded but not yet commited
		finalLink, err := protonDrive.getLink(ctx, linkID)
		if err != nil {
			return nil, 0, err
		}
		return finalLink, 0, nil
	}

	/* step 3: mark the file as active by commiting the revision */
	err = protonDrive.commitNewRevision(ctx, newNodeKR, modTime, fileSize, manifestSignature, linkID, revisionID)
	if err != nil {
		return nil, 0, err
	}

	finalLink, err := protonDrive.getLink(ctx, linkID)
	if err != nil {
		return nil, 0, err
	}
	return finalLink, fileSize, nil
}

/*
There is a route that proton-go-api doesn't have - checkAvailableHashes.
This is used to quickly find the next available filename when the originally supplied filename is taken in the current folder.

Based on the code below, which is taken from the Proton iOS Drive app, we can infer that:
- when a file is to be uploaded && there is filename conflict after the first upload:
	- on web, user will be prompted with a) overwrite b) keep both by appending filename with iteration number c) do nothing
- on the iOS client logic, we can see that when the filename conflict happens (after the upload attampt failed)
	- the filename will be hashed by using filename + iteration
	- 10 iterations will be done per batch, each iteration's hash will be sent to the server
	- the server will return available hashes, and the client will take the lowest iteration as the filename to be used
	- will be used to search for the next available filename (using hashes avoids the filename being known to the server)
*/
