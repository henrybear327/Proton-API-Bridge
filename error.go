package proton_api_bridge

import "errors"

var (
	ErrMainSharePreconditionsFailed          = errors.New("the main share assumption has failed")
	ErrDataFolderNameIsEmpty                 = errors.New("please supply a DataFolderName to enabling file downloading")
	ErrLinkTypeMustToBeFolderType            = errors.New("the link type must be of folder type")
	ErrLinkTypeMustToBeFileType              = errors.New("the link type must be of file type")
	ErrFolderIsNotEmpty                      = errors.New("folder can't be deleted becuase it is not empty")
	ErrInternalErrorOnFileUpload             = errors.New("either link or createFileResp must be not nil")
	ErrMissingInputUploadAndCollectBlockData = errors.New("missing either session key or key ring")
	ErrLinkMustNotBeNil                      = errors.New("Missing input proton link")
)
