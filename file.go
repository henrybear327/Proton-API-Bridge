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
	link, err := protonDrive.c.GetLink(ctx, protonDrive.MainShare.ShareID, linkID)
	if err != nil {
		return nil, nil, err
	}

	return protonDrive.DownloadFile(ctx, &link)
}

func (protonDrive *ProtonDrive) GetRevision(ctx context.Context, link *proton.Link, revisionType proton.RevisionState) (*proton.RevisionMetadata, error) {
	if revisionType != proton.RevisionStateActive && revisionType != proton.RevisionStateDraft {
		// since we can't return more than 1 revision, we only support active and draft types
		return nil, ErrWrongGetRevisionUsage
	}

	revisions, err := protonDrive.c.ListRevisions(ctx, protonDrive.MainShare.ShareID, link.LinkID)
	if err != nil {
		return nil, err
	}
	// log.Printf("revisions %#v", revisions)

	// Revisions are only for files, they represent “versions” of files.
	// Each file can have 1 active/draft revision and n obsolete revisions.
	targetRevision := -1
	for i := range revisions {
		if revisions[i].State == revisionType {
			targetRevision = i
			break
		}
	}
	if targetRevision == -1 { // not found
		return nil, nil
	}

	return &revisions[targetRevision], nil
}

func (protonDrive *ProtonDrive) GetActiveRevisionWithAttrs(ctx context.Context, link *proton.Link) (*proton.Revision, *FileSystemAttrs, error) {
	if link == nil {
		return nil, nil, ErrLinkMustNotBeNil
	}

	revisionMetadata, err := protonDrive.GetRevision(ctx, link, proton.RevisionStateActive)
	if err != nil {
		return nil, nil, err
	}

	revision, err := protonDrive.c.GetRevisionAllBlocks(ctx, protonDrive.MainShare.ShareID, link.LinkID, revisionMetadata.ID)
	if err != nil {
		return nil, nil, err
	}
	// log.Println("Total blocks", len(revision.Blocks))

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

func (protonDrive *ProtonDrive) UploadFileByReader(ctx context.Context, parentLinkID string, filename string, modTime time.Time, file io.Reader, createFileOnly bool) (*proton.Link, int64, error) {
	parentLink, err := protonDrive.c.GetLink(ctx, protonDrive.MainShare.ShareID, parentLinkID)
	if err != nil {
		return nil, 0, err
	}

	return protonDrive.uploadFile(ctx, &parentLink, filename, modTime, file, createFileOnly)
}

func (protonDrive *ProtonDrive) UploadFileByPath(ctx context.Context, parentLink *proton.Link, filename string, filePath string, createFileOnly bool) (*proton.Link, int64, error) {
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

	return protonDrive.uploadFile(ctx, parentLink, filename, info.ModTime(), in, createFileOnly)
}

func (protonDrive *ProtonDrive) createFileUploadDraft(ctx context.Context, parentLink *proton.Link, filename string, modTime time.Time, mimeType string) (*proton.Link, *proton.CreateFileRes, *crypto.SessionKey, *crypto.KeyRing, error) {
	parentNodeKR, err := protonDrive.getNodeKR(ctx, parentLink)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	newNodeKey, newNodePassphraseEnc, newNodePassphraseSignature, err := generateNodeKeys(parentNodeKR, protonDrive.AddrKR)
	if err != nil {
		return nil, nil, nil, nil, err
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
		return nil, nil, nil, nil, err
	}

	parentHashKey, err := parentLink.GetHashKey(parentNodeKR)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	newNodeKR, err := getKeyRing(parentNodeKR, protonDrive.AddrKR, newNodeKey, newNodePassphraseEnc, newNodePassphraseSignature)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	err = createFileReq.SetHash(filename, parentHashKey)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	newSessionKey, err := createFileReq.SetContentKeyPacketAndSignature(newNodeKR, protonDrive.AddrKR)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	createFileResp, err := protonDrive.c.CreateFile(ctx, protonDrive.MainShare.ShareID, createFileReq)
	if err != nil {
		if err == proton.ErrFileNameExist { // FIXME: check for duplicated filename by relying on checkAvailableHashes
			link, err := protonDrive.SearchByNameInFolder(ctx, parentLink, filename, true, false, true) // we search for everything with the requested name in the folder
			if err != nil {
				return nil, nil, nil, nil, err
			}
			return link, nil, nil, nil, nil
		}

		// other real error caught
		return nil, nil, nil, nil, err
	}

	return nil, &createFileResp, newSessionKey, newNodeKR, nil
}

func (protonDrive *ProtonDrive) uploadAndCollectBlockData(ctx context.Context, newSessionKey *crypto.SessionKey, newNodeKR *crypto.KeyRing, file io.Reader, linkID, revisionID string) ([]byte, []proton.BlockToken, int64, error) {
	type PendingUploadBlocks struct {
		blockUploadInfo proton.BlockUploadInfo
		encData         []byte
	}

	if newSessionKey == nil || newNodeKR == nil {
		return nil, nil, 0, ErrMissingInputUploadAndCollectBlockData
	}

	totalFileSize := int64(0)

	blocks := make([]PendingUploadBlocks, 0)
	manifestSignatureData := make([]byte, 0)
	for i := 1; ; i++ {
		// read at most data of size UPLOAD_BLOCK_SIZE
		data := make([]byte, UPLOAD_BLOCK_SIZE) // FIXME: get block size from the server config instead of hardcoding it
		readBytes, err := file.Read(data)
		if err != nil {
			if err == io.EOF {
				if readBytes > 0 {
					log.Fatalln("We have a problem in the assumption")
				}
				break
			}
			return nil, nil, 0, err
		}
		data = data[:readBytes]
		totalFileSize += int64(readBytes)

		// encrypt data
		dataPlainMessage := crypto.NewPlainMessage(data)
		encData, err := newSessionKey.Encrypt(dataPlainMessage)
		if err != nil {
			return nil, nil, 0, err
		}

		encSignature, err := protonDrive.AddrKR.SignDetachedEncrypted(dataPlainMessage, newNodeKR)
		if err != nil {
			return nil, nil, 0, err
		}
		encSignatureStr, err := encSignature.GetArmored()
		if err != nil {
			return nil, nil, 0, err
		}

		h := sha256.New()
		h.Write(encData)
		hash := h.Sum(nil)
		base64Hash := base64.StdEncoding.EncodeToString(hash)
		if err != nil {
			return nil, nil, 0, err
		}
		manifestSignatureData = append(manifestSignatureData, hash...)

		blocks = append(blocks, PendingUploadBlocks{
			blockUploadInfo: proton.BlockUploadInfo{
				Index:        i, // iOS drive: BE starts with 1
				Size:         int64(len(encData)),
				EncSignature: encSignatureStr,
				Hash:         base64Hash,
			},
			encData: encData,
		})
	}

	blockTokens := make([]proton.BlockToken, 0)
	if len(blocks) == 0 {
		return manifestSignatureData, blockTokens, 0, nil
	}

	blockList := make([]proton.BlockUploadInfo, 0)
	for i := range blocks {
		blockList = append(blockList, blocks[i].blockUploadInfo)
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
		return nil, nil, 0, err
	}

	for i := range blockUploadResp {
		err := protonDrive.c.UploadBlock(ctx, blockUploadResp[i].BareURL, blockUploadResp[i].Token, bytes.NewReader(blocks[i].encData))
		if err != nil {
			return nil, nil, 0, err
		}

		blockTokens = append(blockTokens, proton.BlockToken{
			Index: i + 1,
			Token: blockUploadResp[i].Token,
		})
	}

	return manifestSignatureData, blockTokens, totalFileSize, nil
}

func (protonDrive *ProtonDrive) commitNewRevision(ctx context.Context, nodeKR *crypto.KeyRing, modificationTime time.Time, size int64, manifestSignatureData []byte, blockTokens []proton.BlockToken, linkID, revisionID string) error {
	manifestSignature, err := protonDrive.AddrKR.SignDetached(crypto.NewPlainMessage(manifestSignatureData))
	if err != nil {
		return err
	}
	manifestSignatureString, err := manifestSignature.GetArmored()
	if err != nil {
		return err
	}

	updateRevisionReq := proton.UpdateRevisionReq{
		BlockList:         blockTokens,
		State:             proton.RevisionStateActive,
		ManifestSignature: manifestSignatureString,
		SignatureAddress:  protonDrive.signatureAddress,
	}
	err = updateRevisionReq.SetEncXAttrString(protonDrive.AddrKR, nodeKR, modificationTime, size)
	if err != nil {
		return err
	}

	err = protonDrive.c.UpdateRevision(ctx, protonDrive.MainShare.ShareID, linkID, revisionID, updateRevisionReq)
	if err != nil {
		return err
	}

	return nil
}

func (protonDrive *ProtonDrive) uploadFile(ctx context.Context, parentLink *proton.Link, filename string, modTime time.Time, file io.Reader, createFileOnly bool) (*proton.Link, int64, error) {
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
	link, createFileResp, newSessionKey, newNodeKR, err := protonDrive.createFileUploadDraft(ctx, parentLink, filename, modTime, mimeType)
	if err != nil {
		return nil, 0, err
	}

	linkID := ""
	revisionID := ""

	if link != nil {
		linkID = link.LinkID

		draftRevision, err := protonDrive.GetRevision(ctx, link, proton.RevisionStateDraft)
		if err != nil {
			return nil, 0, err
		}
		if draftRevision != nil {
			if protonDrive.Config.ReplaceExistingDraft {
				// FIXME: double check if this is the correct way of handling this case
				// -> delete the draft revision before progressing since we don't maintain clientUID
				// Question: how do we observe for file upload cancellation -> clientUID?
				revisionID = draftRevision.ID
			} else {
				// if there is a draft, based on the web behavior, it will ask if the user wants to replace the failed upload attempt
				// current behavior, we report an error to not upload the file (conservative)
				return nil, 0, ErrDraftExists
			}
		} else {
			// get a new revision
			newRevision, err := protonDrive.c.CreateRevision(ctx, protonDrive.MainShare.ShareID, linkID)
			if err != nil {
				if err == proton.ErrFileCanNotBeFound {
					// Can happen when trying to create a revision on a file without an active revision
					return nil, 0, err
				}
				return nil, 0, err
			}

			revisionID = newRevision.ID
		}

		// get newSessionKey and newNodeKR
		parentNodeKR, err := protonDrive.getNodeKRByID(ctx, link.ParentLinkID)
		if err != nil {
			return nil, 0, err
		}
		newNodeKR, err = link.GetKeyRing(parentNodeKR, protonDrive.AddrKR)
		if err != nil {
			return nil, 0, err
		}
		newSessionKey, err = link.GetSessionKey(protonDrive.AddrKR, newNodeKR)
		if err != nil {
			return nil, 0, err
		}
	} else if createFileResp != nil {
		linkID = createFileResp.ID
		revisionID = createFileResp.RevisionID
	} else {
		// should not happen anymore, since the file search will include the draft now
		return nil, 0, ErrInternalErrorOnFileUpload
	}

	/* step 2: upload blocks and collect block data */
	manifestSignature, blockTokens, fileSize, err := protonDrive.uploadAndCollectBlockData(ctx, newSessionKey, newNodeKR, file, linkID, revisionID)
	if err != nil {
		return nil, 0, err
	}

	if createFileOnly {
		// we try to simulate only draft is created but no upload is performed yet
		finalLink, err := protonDrive.c.GetLink(ctx, protonDrive.MainShare.ShareID, linkID)
		if err != nil {
			return nil, 0, err
		}
		return &finalLink, fileSize, nil
	}

	/* step 3: mark the file as active by updating the revision */
	err = protonDrive.commitNewRevision(ctx, newNodeKR, modTime, fileSize, manifestSignature, blockTokens, linkID, revisionID)
	if err != nil {
		return nil, 0, err
	}

	finalLink, err := protonDrive.c.GetLink(ctx, protonDrive.MainShare.ShareID, linkID)
	if err != nil {
		return nil, 0, err
	}
	return &finalLink, fileSize, nil
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
