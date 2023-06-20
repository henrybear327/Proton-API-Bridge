package proton_api_bridge

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"os"
	"time"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/gabriel-vasile/mimetype"
	"github.com/henrybear327/go-proton-api"
)

func (protonDrive *ProtonDrive) DownloadFileByID(ctx context.Context, linkID string) ([]byte, error) {
	link, err := protonDrive.c.GetLink(ctx, protonDrive.MainShare.ShareID, linkID)
	if err != nil {
		return nil, err
	}

	return protonDrive.DownloadFile(ctx, &link)
}

func (protonDrive *ProtonDrive) DownloadFile(ctx context.Context, link *proton.Link) ([]byte, error) {
	if link.Type != proton.LinkTypeFile {
		return nil, ErrLinkTypeMustToBeFileType
	}

	parentNodeKR, err := protonDrive.getNodeKRByID(ctx, link.ParentLinkID)
	if err != nil {
		return nil, err
	}

	nodeKR, err := link.GetKeyRing(parentNodeKR, protonDrive.AddrKR)
	if err != nil {
		return nil, err
	}

	sessionKey, err := link.GetSessionKey(protonDrive.AddrKR, nodeKR)
	if err != nil {
		return nil, err
	}

	revisions, err := protonDrive.c.ListRevisions(ctx, protonDrive.MainShare.ShareID, link.LinkID)
	if err != nil {
		return nil, err
	}
	// log.Printf("revisions %#v", revisions)

	// Revisions are only for files, they represent “versions” of files.
	// Each file can have 1 active revision and n obsolete revisions.
	activeRevision := -1
	for i := range revisions {
		if revisions[i].State == proton.RevisionStateActive {
			activeRevision = i
		}
	}

	// FIXME: compute total blocks required
	// TODO: handle large file downloading
	revision, err := protonDrive.c.GetRevision(ctx, protonDrive.MainShare.ShareID, link.LinkID, revisions[activeRevision].ID, 1, 50)
	if err != nil {
		return nil, err
	}

	buffer := bytes.NewBuffer(nil)
	for i := range revision.Blocks {
		// parallel download
		blockReader, err := protonDrive.c.GetBlock(ctx, revision.Blocks[i].BareURL, revision.Blocks[i].Token)
		if err != nil {
			return nil, err
		}
		defer blockReader.Close()

		err = decryptBlockIntoBuffer(sessionKey, protonDrive.AddrKR, nodeKR, revision.Blocks[i].EncSignature, buffer, blockReader)
		if err != nil {
			return nil, err
		}
	}

	return buffer.Bytes(), nil
}

func (protonDrive *ProtonDrive) UploadFileByReader(ctx context.Context, parentLinkID string, filename string, modTime time.Time, file io.Reader) (*proton.Link, error) {
	parentLink, err := protonDrive.c.GetLink(ctx, protonDrive.MainShare.ShareID, parentLinkID)
	if err != nil {
		return nil, err
	}

	return protonDrive.uploadFile(ctx, &parentLink, filename, time.Now() /* FIXME */, file)
}

func (protonDrive *ProtonDrive) UploadFileByPath(ctx context.Context, parentLink *proton.Link, filename string, filePath string) (*proton.Link, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	in := bufio.NewReader(f)

	return protonDrive.uploadFile(ctx, parentLink, filename, info.ModTime(), in)
}

func (protonDrive *ProtonDrive) uploadFile(ctx context.Context, parentLink *proton.Link, filename string, modTime time.Time, file io.Reader) (*proton.Link, error) {
	// FIXME: check iOS: optimize for large files -> enc blocks on the fly
	/*
		Assumptions:
		- Upload is always done to the mainShare
	*/
	// TODO: check for duplicated filename by using checkAvailableHashes

	parentNodeKR, err := protonDrive.getNodeKR(ctx, parentLink)
	if err != nil {
		return nil, err
	}

	// detect MIME type
	fileContent, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	mimetype.SetLimit(0)
	mType := mimetype.Detect(fileContent)
	mimeType := mType.String()
	// log.Println("Detected MIME type", mimeType)

	/* step 1: create a draft */
	newNodeKey, newNodePassphraseEnc, newNodePassphraseSignature, err := generateNodeKeys(parentNodeKR, protonDrive.AddrKR)
	if err != nil {
		return nil, err
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

		ModifyTime: modTime.Unix(), // The modified time

		SignatureAddress: protonDrive.signatureAddress, // Signature email address used to sign passphrase and name
	}

	/* Name is encrypted using the parent's keyring, and signed with address key */
	err = createFileReq.SetName(filename, protonDrive.AddrKR, parentNodeKR)
	if err != nil {
		return nil, err
	}

	parentHashKey, err := parentLink.GetHashKey(parentNodeKR)
	if err != nil {
		return nil, err
	}

	newNodeKR, err := getKeyRing(parentNodeKR, protonDrive.AddrKR, newNodeKey, newNodePassphraseEnc, newNodePassphraseSignature)
	if err != nil {
		return nil, err
	}

	err = createFileReq.SetHash(filename, parentHashKey)
	if err != nil {
		return nil, err
	}

	err = createFileReq.SetContentKeyPacketAndSignature(newNodeKR, protonDrive.AddrKR)
	if err != nil {
		return nil, err
	}

	createFileResp, err := protonDrive.c.CreateFile(ctx, protonDrive.MainShare.ShareID, createFileReq)
	if err != nil {
		return nil, err
	}

	if len(fileContent) == 0 {
		/* step 2 [Skipped]: upload blocks and collect block data */

		/* step 3: mark the file as active by updating the revision */
		manifestSignatureData := make([]byte, 0)
		manifestSignature, err := protonDrive.AddrKR.SignDetached(crypto.NewPlainMessage(manifestSignatureData))
		if err != nil {
			return nil, err
		}
		manifestSignatureString, err := manifestSignature.GetArmored()
		if err != nil {
			return nil, err
		}

		err = protonDrive.c.UpdateRevision(ctx, protonDrive.MainShare.ShareID, createFileResp.ID, createFileResp.RevisionID, proton.UpdateRevisionReq{
			BlockList:         make([]proton.BlockToken, 0),
			State:             proton.RevisionStateActive,
			ManifestSignature: manifestSignatureString,
			SignatureAddress:  protonDrive.signatureAddress,
		})
		if err != nil {
			return nil, err
		}
	} else {
		/* step 2: upload blocks and collect block data */
		// FIXME: handle partial upload (failed midway)

		// FIXME: get block size
		blockSize := 4 * 1024 * 1024
		type PendingUploadBlocks struct {
			blockUploadInfo proton.BlockUploadInfo
			encData         []byte
		}
		blocks := make([]PendingUploadBlocks, 0)
		manifestSignatureData := make([]byte, 0)
		sessionKey, err := func() (*crypto.SessionKey, error) {
			keyPacket := createFileReq.ContentKeyPacket
			keyPacketByteArr, err := base64.StdEncoding.DecodeString(keyPacket)
			if err != nil {
				return nil, err
			}

			sessionKey, err := newNodeKR.DecryptSessionKey(keyPacketByteArr)
			if err != nil {
				return nil, err
			}

			// FIXME: verify the signature of the session key
			// signatureString, err := crypto.NewPGPMessageFromArmored(createFileReq.ContentKeyPacketSignature)
			// if err != nil {
			// 	return nil, err
			// }

			// err = protonDrive.AddrKR.VerifyDetachedEncrypted(crypto.NewPlainMessageFromString(sessionKey.GetBase64Key()), signatureString, newNodeKR, crypto.GetUnixTime())
			// if err != nil {
			// 	return nil, err
			// }

			return sessionKey, nil
		}()
		if err != nil {
			return nil, err
		}

		for i := 0; i*blockSize < len(fileContent); i++ {
			// encrypt data
			upperBound := (i + 1) * blockSize
			if upperBound > len(fileContent) {
				upperBound = len(fileContent)
			}
			data := fileContent[i*blockSize : upperBound]

			dataPlainMessage := crypto.NewPlainMessage(data)
			encData, err := sessionKey.Encrypt(dataPlainMessage)
			if err != nil {
				return nil, err
			}

			encSignature, err := protonDrive.AddrKR.SignDetachedEncrypted(dataPlainMessage, newNodeKR)
			if err != nil {
				return nil, err
			}
			encSignatureStr, err := encSignature.GetArmored()
			if err != nil {
				return nil, err
			}

			h := sha256.New()
			h.Write(encData)
			hash := h.Sum(nil)
			base64Hash := base64.StdEncoding.EncodeToString(hash)
			if err != nil {
				return nil, err
			}
			manifestSignatureData = append(manifestSignatureData, hash...)

			blocks = append(blocks, PendingUploadBlocks{
				blockUploadInfo: proton.BlockUploadInfo{
					Index:        i + 1, // iOS drive: BE starts with 1
					Size:         int64(len(encData)),
					EncSignature: encSignatureStr,
					Hash:         base64Hash,
				},
				encData: encData,
			})
		}

		blockList := make([]proton.BlockUploadInfo, 0)
		for i := 0; i < len(blocks); i++ {
			blockList = append(blockList, blocks[i].blockUploadInfo)
		}
		blockTokens := make([]proton.BlockToken, 0)
		blockUploadReq := proton.BlockUploadReq{
			AddressID:  protonDrive.MainShare.AddressID,
			ShareID:    protonDrive.MainShare.ShareID,
			LinkID:     createFileResp.ID,
			RevisionID: createFileResp.RevisionID,

			BlockList: blockList,
		}
		blockUploadResp, err := protonDrive.c.RequestBlockUpload(ctx, blockUploadReq)
		if err != nil {
			return nil, err
		}

		for i := range blockUploadResp {
			err := protonDrive.c.UploadBlock(ctx, blockUploadResp[i].BareURL, blockUploadResp[i].Token, bytes.NewReader(blocks[i].encData))
			if err != nil {
				return nil, err
			}

			blockTokens = append(blockTokens, proton.BlockToken{
				Index: i + 1,
				Token: blockUploadResp[i].Token,
			})
		}

		/* step 3: mark the file as active by updating the revision */

		// TODO: check iOS Drive CommitableRevision
		manifestSignature, err := protonDrive.AddrKR.SignDetached(crypto.NewPlainMessage(manifestSignatureData))
		if err != nil {
			return nil, err
		}
		manifestSignatureString, err := manifestSignature.GetArmored()
		if err != nil {
			return nil, err
		}

		err = protonDrive.c.UpdateRevision(ctx, protonDrive.MainShare.ShareID, createFileResp.ID, createFileResp.RevisionID, proton.UpdateRevisionReq{
			BlockList:         blockTokens,
			State:             proton.RevisionStateActive,
			ManifestSignature: manifestSignatureString,
			SignatureAddress:  protonDrive.signatureAddress,
		})
		if err != nil {
			return nil, err
		}
	}

	link, err := protonDrive.c.GetLink(ctx, protonDrive.MainShare.ShareID, createFileResp.ID)
	if err != nil {
		return nil, err
	}
	return &link, nil
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

private func findNextAvailableName(for file: FileNameCheckerModel, offset: Int, completion: @escaping (Result<NameHashPair, Error>) -> Void) {
	assert(offset >= 0)
	let fileName = file.originalName.fileName()
	let `extension` = file.originalName.fileExtension()
	var possibleNamesHashPairs = [NameHashPair]()

	let lowerBound = offset + 1
	let upperBound = offset + step

	for iteration in lowerBound...upperBound {
		let newName = "\(fileName) (\(iteration))" + (`extension`.isEmpty ? "" : "." + `extension`)
		guard let newHash = try? hasher(newName, file.parentNodeHashKey) else { continue }
		possibleNamesHashPairs.append(NameHashPair(name: newName, hash: newHash))
	}

	hashChecker.checkAvailableHashes(among: possibleNamesHashPairs, onFolder: file.parent) { [weak self] result in
		guard let self = self else { return }

		switch result {
		case .failure(let error):
			completion(.failure(error))

		case .success(let approvedHashes) where approvedHashes.isEmpty:
			self.findNextAvailableName(for: file, offset: upperBound, completion: completion)

		case .success(let approvedHashes):
			let approvedPair = possibleNamesHashPairs.first { approvedHashes.contains($0.hash) }!
			completion(.success(approvedPair))
		}
	}
}
*/
