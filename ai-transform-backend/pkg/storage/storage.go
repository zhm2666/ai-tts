package storage

type Storage interface {
	DownloadFile(objectKey string, dstPath string) error
	UploadFromFile(path string, dstPath string) (url string, err error)
}

type StorageFactory interface {
	CreateStorage() Storage
}
