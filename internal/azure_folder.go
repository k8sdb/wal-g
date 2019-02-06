package internal

import (
	"bytes"
	"context"
	"fmt"
	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/pkg/errors"
	"github.com/tamalsaha/wal-g-demo/tracelog"
	"io"
	"net/url"
	"os"
	"strings"
	"time"
)

type AzureFolderError struct {
	error
}

func NewAzureFolderError(err error, format string, args ...interface{}) AzureFolderError {
	return AzureFolderError{errors.Wrapf(err, format, args...)}
}

func (err AzureFolderError) Error() string {
	return fmt.Sprintf(tracelog.GetErrorFormatter(), err.error)
}

func NewAzureFolder(containerURL azblob.ContainerURL, path string) *AzureFolder {
	return &AzureFolder{containerURL, path}
}

func ConfigureAzureFolder(prefix string) (StorageFolder, error) {
	accountName, accountKey := accountInfo()
	if len(accountName) == 0 || len(accountKey) == 0 {
		return nil, NewAzureFolderError(errors.New("Credential error"),
			"Either the AZURE_STORAGE_ACCOUNT or AZURE_STORAGE_ACCESS_KEY environment variable is not set")
	}
	credential, err := azblob.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		return nil, NewAzureFolderError(err, "Unable to create Azure credentials")
	}
	pipeLine := azblob.NewPipeline(credential, azblob.PipelineOptions{})
	containerName, path, err := getPathFromPrefix(prefix)
	if err != nil {
		return nil, NewAzureFolderError(err, "Unable to create Azure container")
	}
	serviceURL, err := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/%s", accountName,containerName))
	if err != nil {
		return nil, NewAzureFolderError(err, "Unable to parse Azure service URL")
	}
	containerURL := azblob.NewContainerURL(*serviceURL, pipeLine)
	path = addDelimiterToAzPath(path)
	return NewAzureFolder(containerURL, path), nil
}

func addDelimiterToAzPath(path string) string {
	if strings.HasSuffix(path, "/") || path == "" {
		return path
	}
	return path + "/"
}

type AzureFolder struct {
	containerURL azblob.ContainerURL
	path         string
}

func (folder *AzureFolder) GetPath() string {
	return folder.path
}

func (folder *AzureFolder) Exists(objectRelativePath string) (bool, error) {
	path := JoinS3Path(folder.path, objectRelativePath)
	fmt.Println("folder path = ",folder.path)
	ctx := context.Background()
	blobURL := folder.containerURL.NewBlockBlobURL(path)
	_,err := blobURL.GetProperties(ctx, azblob.BlobAccessConditions{})
	if stgErr, ok := err.(azblob.StorageError); ok && stgErr.ServiceCode() == azblob.ServiceCodeBlobNotFound{
		return false, nil
	}
	if err != nil {
		return false, NewAzureFolderError(err, "Unable to stat object %v", path)
	}
	return true, nil
}

func (folder *AzureFolder) ListFolder() (objects []StorageObject, subFolders []StorageFolder, err error) {
	//Marker is used for segmentation purposes.
	for marker := (azblob.Marker{}); marker.NotDone(); {
		blobs, err := folder.containerURL.ListBlobsHierarchySegment(context.Background(), marker,"/", azblob.ListBlobsSegmentOptions{Prefix:folder.path})
		if err != nil {
			return nil,nil, NewAzureFolderError(err, "Unable to iterate %v",folder.path)
		}
		for _, blob := range blobs.Segment.BlobItems{
			objName := strings.TrimPrefix(blob.Name, folder.path)
			updated := time.Time(blob.Properties.LastModified)
			objects = append(objects, &AzureStorageObject{updated, objName})
		}

		marker = blobs.NextMarker
		fmt.Println()
		blobPrefixes := blobs.Segment.BlobPrefixes
		for _,blobPrefix := range blobPrefixes{
			subFolderPath := blobPrefix.Name
			subFolders = append(subFolders, NewAzureFolder(folder.containerURL, subFolderPath))
		}

	}
	return
}

func (folder *AzureFolder) GetSubFolder(subFolderRelativePath string) StorageFolder {
	return NewAzureFolder(folder.containerURL, addDelimiterToAzPath(JoinS3Path(folder.path, subFolderRelativePath)))
}

func (folder *AzureFolder) ReadObject(objectRelativePath string) (io.ReadCloser, error) {
	path := JoinS3Path(folder.path, objectRelativePath)
	blobURL := folder.containerURL.NewBlockBlobURL(path)
	downloadResponse, err := blobURL.Download(context.Background(),0,0,azblob.BlobAccessConditions{},false)
	if err != nil {
		return nil,NewAzureFolderError(err, "Unable to download blob %s.", path)
	}
	content := downloadResponse.Body(azblob.RetryReaderOptions{})
	return content, nil
}

func (folder *AzureFolder) PutObject(name string, content io.Reader) error {
	tracelog.DebugLogger.Printf("Put %v into %v\n", name, folder.path)
	path := JoinS3Path(folder.path, name)
	blobURL := folder.containerURL.NewBlockBlobURL(path)
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(content)
	if err != nil {
		return NewAzureFolderError(err, "Unable to copy to object")
	}
	uploadContent := bytes.NewReader(buf.Bytes())
	_ , err = blobURL.Upload(context.Background(), uploadContent ,azblob.BlobHTTPHeaders{ContentType: "text/plain"}, azblob.Metadata{}, azblob.BlobAccessConditions{})
	if err != nil{
		return NewAzureFolderError(err, "unable to upload blob %v", name)
	}
	tracelog.DebugLogger.Printf("Put %v done\n", name)
	return nil
}

func (folder *AzureFolder) DeleteObjects(objectRelativePaths []string) error {
	for _, objectRelativePath := range objectRelativePaths {
		path := JoinS3Path(folder.path, objectRelativePath)
		blobURL := folder.containerURL.NewBlockBlobURL(path)
		tracelog.DebugLogger.Printf("Delete %v\n", path)
		_, err := blobURL.Delete(context.Background(),azblob.DeleteSnapshotsOptionInclude,azblob.BlobAccessConditions{})
		if stgErr, ok := err.(azblob.StorageError); ok && stgErr.ServiceCode() == azblob.ServiceCodeBlobNotFound{
			continue
		}
		if err != nil{
			return NewAzureFolderError(err,"Unable to delete object %v", path)
		}
	}
	return nil
}

func accountInfo() (string, string) {
	accountName := os.Getenv("AZURE_STORAGE_ACCOUNT")
	accountKey := os.Getenv("AZURE_STORAGE_ACCESS_KEY")
	return accountName, accountKey
}