package images

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"

	"github.com/discuitnet/discuit/internal/uid"
)

type s3Store struct{}

func newS3Store() *s3Store {
	return &s3Store{}
}

func (ds *s3Store) name() string {
	return "s3"
}

// TODO: use minio and implement the s3Store methods

func (ds *s3Store) get(r *ImageRecord) ([]byte, error) {
	fmt.Println("s3Store.get")
	filepath, err := ds.imagePath(r.ID, r.Format)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath)
	if err != nil {
		err = fmt.Errorf("get image %v: %w", r.ID, err)
	}
	return data, err
}

// imagePath returns the path p of where the image should be stored. It creates
// the residing folder, and all parent folders, if they're not found.
func (ds *s3Store) imagePath(imageID uid.ID, f ImageFormat) (p string, err error) {
	folder, filename := idToFolder(imageID)
	folder = path.Join(filesRootFolder, folder)
	if err = mkdirAll(folder); err != nil {
		return
	}
	filename += f.Extension()
	p = path.Join(folder, filename)
	return
}

func (ds *s3Store) save(r *ImageRecord, image []byte) error {
	filepath, err := ds.imagePath(r.ID, r.Format)
	if err != nil {
		return fmt.Errorf("error creating images folder: %v", err)
	}
	if err := os.WriteFile(filepath, image, 0755); err != nil {
		return fmt.Errorf("error writing image file %v: %v", filepath, err)
	}
	return nil
}

func (ds *s3Store) delete(r *ImageRecord) error {
	filepath, err := ds.imagePath(r.ID, r.Format)
	if err != nil {
		return err
	}
	err = os.Remove(filepath)
	if errors.Is(err, fs.ErrNotExist) {
		// Image does not exist for some reason. Could be because of a failed
		// delete image transaction earlier.
		return nil
	}
	return err
}
