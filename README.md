# Proton API Bridge

Thanks to Proton open sourcing [proton-go-api](https://github.com/ProtonMail/go-proton-api) and the web, iOS, and Android client codebases, we don't need to completely reverse engineer the APIs.

[proton-go-api](https://github.com/ProtonMail/go-proton-api) provides the basic building blocks of API calls and error handling, such as 429 exponential back-off, but it is pretty much just a barebone interface to the Proton API. For example, the encryption and decryption of the Proton Drive file are not provided in this library. 

This codebase, Proton API Bridge, bridges the gap, so software like [rclone](https://github.com/rclone/rclone) can be built on top of this quickly. This codebase handles the intricate tasks before and after calling Proton APIs, particularly the complex encryption scheme, allowing developers to implement features for other software on top of this codebase.

Currently, only Proton Drive APIs are bridged, as we are aiming to implement a backend for rclone.

## Sidenotes

We are using a fork of the [proton-go-api](https://github.com/henrybear327/go-proton-api), adding quite some new code to it. We will try to commit back to the upstream once we feel like the code changes are stable.

# Instructions to run the code

## Compiling and running

`go run .`

## Unit testing and linting 

`golangci-lint run && go test -race -failfast -v ./...`

# Drive APIs

> In collaboration with Azimjon Pulatov, in memory of our good old days at Meta, London, in the summer of 2022.

Currently, the development are split into 2 versions. V1 supports the features [required by rclone](https://github.com/henrybear327/rclone/blob/master/fs/types.go), such as `file listing`. V2 will support the extra features that rclone has interface for, such as `move file` and `move folder` operations.

## V1

### Features

- [x] Log in to an account without 2FA using username and password 
- [x] Obtain keyring
- [x] Cache access token, etc. to be able to reuse the session
    - [x] Bug: 403: Access token does not have sufficient scope - used the wrong newClient function
- [x] Volume actions
    - [x] List all volumes
- [x] Share actions
    - [x] Get all shares
    - [x] Get default share
- [x] Fix context with proper propagation instead of using `ctx` everywhere
- [x] Folder actions
    - [x] List all folders and files within the root folder
        - [x] BUG: listing directory - missing signature when there are more than 1 share
            - maybe the way I decrypt the keyring is wrong
                - (wrong fix for the first time) bug on no name for the root folder 
                - (correct fix) we need to check for the "active" folder type first
    - [x] List all folders and files recursively within the root folder
    - [x] Delete
        - [x] Implement delete all for testing -> very dangerous, thus currently guarded with a hardcoded string
    - [x] Create
- [ ] File actions
    - [x] Download
        - [x] Download empty file
        - [ ] Properly handle large files and empty files (check iOS codebase)
            - esp. large files, where buffering in-memory will screw up the runtime
        - [ ] Check signature and hash
    - [x] Delete
    - [x] Upload
        - [x] Handle empty file        
        - [x] Parse mime type 
        - [ ] Force overwrite
        - [ ] Add revision
        - [ ] Improve to handle large files
        - [ ] Upload verification
        - [ ] Handle failed / interrupted upload
        - [x] Modified time
    - [ ] List file metadata 
- [ ] Duplicated file/folder name handling: 422: A file or folder with that name already exists (Code=2500, Status=422)
- [ ] Handle ERROR RESTY 422: File or folder was not found. (Code=2501, Status=422), Attempt 1
- [x] Init ProtonDrive with config passed in as Map
- [x] Remove all `log.Fatalln` and use proper error propagation (basically remove `HandleError` and we go from there)
- [x] Integration tests
    - [x] Remove drive demo code
    - [x] Create a Drive struct to encapsulate all the functions (maybe?)
    - [x] Move comments to proper places
    - [x] Modify `shouldRejectDestructiveActions()`
    - [ ] Check file metadata
    - [ ] Try to check if all functions are used at least once so we know if it's functioning or not
- [ ] Documentation
- [x] Reduce config options on caching access token
- [x] Remove integration test safeguarding
- [ ] Improve file searching function to use HMAC instead of just using string comparison
- [ ] Remove e.g. proton.link related exposures in the function signature (this library should abstract them all)

### TODO

- [x] address go dependencies
    - Fixed by doing the following in the `go-proton-api` repo to bump to use the latest commit
        - `go get github.com/ProtonMail/go-proton-api@ea8de5f674b7f9b0cca8e3a5076ffe3c5a867e01`
        - `go get github.com/ProtonMail/gluon@fb7689b15ae39c3efec3ff3c615c3d2dac41cec8`
- [x] Remove mail-related apis (to reduce dependencies) 
- [x] Make a "super class" and expose all necessary methods for the outside to call
- [x] Add 2FA login
- [ ] Go through Drive iOS source code and check the logic control flow
- [x] Fix the function argument passing (using pointers)
- [ ] Use proper AppVersion (we need to be friendly to the Proton servers)
- [ ] Handle account with
    - [ ] multiple addresses
    - [ ] multiple keys per addresses
    - [ ] multiple shares 
- [x] Update RClone's contribution.md file
- [x] Remove delete all's hardcoded string
- [ ] Address TODO and FIXME
- [ ] Use CI to run integration tests
- [ ] Figure out the bottleneck by doing some profiling 
- [ ] Some error handling from [here](https://github.com/ProtonMail/WebClients/blob/main/packages/shared/lib/drive/constants.ts) MAX_NAME_LENGTH, TIMEOUT
- [x] Point to the right proton-go-api branch
    - [x] Run `go get github.com/henrybear327/go-proton-api@dev` to update go mod

### Known limitations

- Large file handling: for uploading, files will be loaded into the memory entirely, encrypted, and then chunked; for downloading, the file will be written when all blocks are decrypted and checked
- Crypto-related operations, e.g. signature verification, still needs to cross check with iOS or web open source codebase 
- No move for file and folders, thumbnails, respecting accepted MIME types, max upload size, can't init Proton Drive (coming in V2)
- Assumptions: only one main share per account

## V2

Moving files and folders are [features](https://github.com/rclone/rclone/blob/51a468b2bae4ca8e21760435211623a8199a9167/fs/features.go#L25)

- [ ] Folder
    - [ ] (Feature) Update (force overwrite)
    - [ ] (Feature) Move
- [ ] Commit back to proton-go-api and switch to using upstream (make sure the tag is at the tip though)
- [ ] Support legacy 2-password mode
- [ ] Support thumbnail
- [ ] Proton Drive init (no prior Proton Drive login before -> probably will have no key, volume, etc. to start with at all)
- [ ] linkID caching -> would need to listen to the event api though

# Questions

- [x] rclone's folder / file rename detection? -> just implement the interface and rclone will deal with the rest!
- [ ] How often will we run into 429 on login
