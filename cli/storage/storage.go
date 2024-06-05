package storage

import (
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/discuitnet/discuit/config"
	"github.com/discuitnet/discuit/internal/images"
	msql "github.com/discuitnet/discuit/internal/sql"
	"github.com/discuitnet/discuit/internal/utils"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:  "storage",
	Usage: "Manage storage",
	Subcommands: []*cli.Command{
		{
			Name:  "migrate",
			Usage: "Migrate images between local storage and S3 storage.",
			Subcommands: []*cli.Command{
				{
					Name:  "to-s3",
					Usage: "Migrate images to S3",
					Flags: []cli.Flag{
						&cli.BoolFlag{
							Name:        "clean",
							Usage:       "Clean local images after migration",
							DefaultText: "false",
						},
					},
					Action: func(ctx *cli.Context) error {
						conf := ctx.Context.Value("config").(*config.Config)
						db := ctx.Context.Value("db").(*sql.DB)

						// Get S3 credentials
						endpoint := conf.S3Endpoint
						region := conf.S3Region
						bucket := conf.S3Bucket
						accessKeyID := conf.S3SecretAccessKeyID
						secretAccessKey := conf.S3SecretAccessKey
						useSSL := true

						// Initialize minio client object.
						minioClient, err := minio.New(endpoint, &minio.Options{
							Region: region,
							Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
							Secure: useSSL,
						})
						if err != nil {
							return err
						}

						// Get path to images dir
						p := "images"
						if conf.ImagesFolderPath != "" {
							p = conf.ImagesFolderPath
						}
						p, err = filepath.Abs(p)
						if err != nil {
							return err
						}

						// TODO: Rather than reading the filesystem, call the DB to get the images that need to be uploaded

						// TODO: Wrap in while loop so w get 1000-5000 records, handle image upload, after completion set store_name to disk

						// images, err = db.

						// // Get all images
						// err = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
						// 	if !d.IsDir() {
						// 		imagesToUpload = append(imagesToUpload, path)
						// 	}
						// 	return nil
						// })
						// if err != nil {
						// 	return err
						// }
						// fmt.Printf("Found %d images to upload\n", len(imagesToUpload))

						// Check if the bucket already exists
						bucketExists, err := minioClient.BucketExists(ctx.Context, bucket)
						if err != nil {
							return fmt.Errorf("Failed to check if bucket %s exists: %v", bucket, err)
						}
						if !bucketExists {
							err = minioClient.MakeBucket(ctx.Context, bucket, minio.MakeBucketOptions{Region: region})
							if err != nil {
								// Check to see if we already own this bucket (which happens if you run this twice)
								exists, errBucketExists := minioClient.BucketExists(ctx.Context, bucket)
								if errBucketExists == nil && exists {
									fmt.Printf("We already own %s\n", bucket)
								} else {
									return fmt.Errorf("Failed to create bucket %s: %v", bucket, err)
								}
							} else {
								fmt.Printf("Successfully created %s\n", bucket)

								policy := fmt.Sprintf(`
								{
									"Version": "2012-10-17",
									"Statement": [
										{
											"Effect": "Allow",
											"Principal": {
												"AWS": [
													"*"
												]
											},
											"Action": [
												"s3:GetObject"
											],
											"Resource": [
												"arn:aws:s3:::%v/*"
											]
										}
									]
								}`, bucket, bucket)

								err = minioClient.SetBucketPolicy(ctx.Context, bucket, policy)
								if err != nil {
									return fmt.Errorf("Failed to set policy on %s: %v", bucket, err)
								}

								fmt.Printf("Successfully set policy on %s\n", bucket)

							}
						}

						// Array of the image id's
						var imageIDs []string

						currentStart := 0
						currentLimit := 5000
						imagesStillLeft := true

						successfullUploads := 0
						failedUploads := 0

						for imagesStillLeft {
							// TODO: this is done incorrectly I believe, seems to repeat the same images (sometimes, may not be a real problem)
							records, err := images.GetImageRecordsx(ctx.Context, db, "disk", currentStart, currentLimit)
							if err != nil {
								if err.Error() == "image not found" {
									imagesStillLeft = false
									continue
								}

								return fmt.Errorf("Failed to get image records: %v", err)
							}

							// fmt.Printf("Found %d images to upload\n", len(records))

							for _, record := range records {
								imagesToUpload := []string{}
								hash := sha1.Sum(record.ID.Bytes())
								hex := hex.EncodeToString(hash[:])
								folder := hex[:2] + "/" + hex[2:3]
								filename := hex[3:]
								// objectName := folder + "/" + filename + record.Format.Extension()
								contentType := "image/" + strings.Split(record.Format.Extension(), ".")[1]
								// filePath := filepath.Join(p, folder, filename+record.Format.Extension())
								imagesExistForID := false

								// If the directory doesn't exist, do nothing
								if _, err := os.Stat(filepath.Join(p, folder)); os.IsNotExist(err) {
									continue
								}

								err = filepath.WalkDir(p+"/"+folder, func(path string, d fs.DirEntry, err error) error {
									if !d.IsDir() {
										tbase := filepath.Base(path)
										if strings.Contains(tbase, filename) {
											imagesToUpload = append(imagesToUpload, path)
											imagesExistForID = true
										}
									}
									return nil
								})
								if err != nil {
									return err
								}

								// fmt.Printf("Found %d images to upload in folder %s\n", len(imagesToUpload), folder)

								for _, image := range imagesToUpload {
									objectName := folder + "/" + filepath.Base(image)
									filePath := filepath.Join(p, objectName)

									// Check if the image already exists
									stat, err := minioClient.StatObject(ctx.Context, bucket, objectName, minio.StatObjectOptions{})
									if err != nil {
										if err.Error() == "The specified key does not exist." {
											// Ignore
										} else {
											fmt.Errorf("Failed to check if image %s exists in the bucket: %v", objectName, err)
											continue
										}
									}
									if stat.Size > 0 {
										fmt.Printf("Image %s already exists in the bucket\n", objectName)
										continue
									}

									info, err := minioClient.FPutObject(ctx.Context, bucket, objectName, filePath, minio.PutObjectOptions{ContentType: contentType})
									if err != nil {
										failedUploads++
										fmt.Errorf("Failed to upload %s: %v", objectName, err)
										continue
									}

									successfullUploads++
									log.Printf("Successfully uploaded %s of size %d\n", objectName, info.Size)
								}

								// Add id to the array if it's not already in there
								if imagesExistForID && !utils.StringInSlice(record.ID.String(), imageIDs) {
									imageIDs = append(imageIDs, record.ID.String())
								}

								// TODO: Every X records in imageIDs, update the store_name to s3
							}

							currentStart += currentLimit
						}

						if len(imageIDs) > 0 {
							tx, err := db.BeginTx(ctx.Context, nil)
							if err != nil {
								return fmt.Errorf("Failed to start transaction: %v", err)
							}

							query, args := msql.BuildUpdateQuery("images", []msql.ColumnValue{{Name: "store_name", Value: "s3"}}, fmt.Sprintf("WHERE id IN %s", msql.BuildInClause(imageIDs)))

							if _, err = tx.ExecContext(ctx.Context, query, args...); err != nil {
								if err := tx.Rollback(); err != nil {
									log.Println("images.SaveImage rollback error: ", err)
								}
								return err
							}

							if err = tx.Commit(); err != nil {
								return fmt.Errorf("Failed to commit transaction: %v", err)
							}

							// TODO: Probably should also wipe the imageIDs array when done
						}

						fmt.Printf("Successfully uploaded %d images\n", successfullUploads)
						fmt.Printf("Failed to upload %d images\n", failedUploads)
						fmt.Printf("Total images: %d\n", successfullUploads+failedUploads)

						return nil
					},
				},
				{
					Name:  "to-local",
					Usage: "Migrate images to local storage",
					Flags: []cli.Flag{
						&cli.BoolFlag{
							Name:        "clean",
							Usage:       "Clean S3 images after migration",
							DefaultText: "false",
						},
						&cli.BoolFlag{
							Name:        "force",
							Usage:       "Force download even if the image already exists",
							DefaultText: "false",
						},
					},
					Action: func(ctx *cli.Context) error {
						conf := ctx.Context.Value("config").(*config.Config)
						imagesToDownload := []string{}

						// Get S3 credentials
						endpoint := conf.S3Endpoint
						region := conf.S3Region
						bucket := conf.S3Bucket
						accessKeyID := conf.S3SecretAccessKeyID
						secretAccessKey := conf.S3SecretAccessKey
						useSSL := true

						// Initialize minio client object.
						minioClient, err := minio.New(endpoint, &minio.Options{
							Region: region,
							Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
							Secure: useSSL,
						})
						if err != nil {
							log.Fatalln(err)
						}

						// Get path to images dir
						p := "images"
						if conf.ImagesFolderPath != "" {
							p = conf.ImagesFolderPath
						}
						p, err = filepath.Abs(p)
						if err != nil {
							log.Fatalf("Error attempting to set the images folder location (%s): %v", p, err)
						}

						// Get all images
						for object := range minioClient.ListObjects(ctx.Context, bucket, minio.ListObjectsOptions{Recursive: true}) {
							if object.Err != nil {
								return object.Err
							}
							imagesToDownload = append(imagesToDownload, object.Key)
						}

						fmt.Printf("Found %d images to download\n", len(imagesToDownload))

						for _, image := range imagesToDownload {
							objectName := image
							filePath := filepath.Join(p, objectName)

							// Check if the image already exists
							if _, err := os.Stat(filePath); err == nil && !ctx.Bool("force") {
								fmt.Printf("Image %s already exists\n", objectName)
								continue
							}
							if err != nil && !os.IsNotExist(err) {
								return err
							}

							// Download the image
							err = minioClient.FGetObject(ctx.Context, bucket, objectName, filePath, minio.GetObjectOptions{})
							if err != nil {
								return err
							}

							log.Printf("Successfully downloaded %s\n", objectName)

							if ctx.Bool("clean") {
								err = minioClient.RemoveObject(ctx.Context, bucket, objectName, minio.RemoveObjectOptions{})
								if err != nil {
									return err
								}
								fmt.Printf("Removed %s\n", objectName)
							}
						}

						return nil
					},
				},
			},
		},
		{
			Name:  "clean",
			Usage: "Clean storage",
			Subcommands: []*cli.Command{
				{
					Name:  "local",
					Usage: "Clean local storage",
					Action: func(ctx *cli.Context) error {
						conf := ctx.Context.Value("config").(*config.Config)

						// Get path to images dir
						p := "images"
						if conf.ImagesFolderPath != "" {
							p = conf.ImagesFolderPath
						}
						p, err := filepath.Abs(p)
						if err != nil {
							log.Fatalf("Error attempting to set the images folder location (%s): %v", p, err)
						}

						// Get all images
						err = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
							if !d.IsDir() {
								err = os.Remove(path)
								if err != nil {
									return err
								}
								fmt.Printf("Removed %s\n", path)
							}
							return nil
						})
						if err != nil {
							log.Fatalf("impossible to walk directories: %s", err)
						}

						return nil
					},
				},
				{
					Name:  "s3",
					Usage: "Clean S3 storage",
					Action: func(ctx *cli.Context) error {
						conf := ctx.Context.Value("config").(*config.Config)

						// Get S3 credentials
						endpoint := conf.S3Endpoint
						region := conf.S3Region
						bucket := conf.S3Bucket
						accessKeyID := conf.S3SecretAccessKeyID
						secretAccessKey := conf.S3SecretAccessKey
						useSSL := true

						// Initialize minio client object.
						minioClient, err := minio.New(endpoint, &minio.Options{
							Region: region,
							Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
							Secure: useSSL,
						})
						if err != nil {
							log.Fatalln(err)
						}

						// Get all images
						for object := range minioClient.ListObjects(ctx.Context, bucket, minio.ListObjectsOptions{Recursive: true}) {
							if object.Err != nil {
								return object.Err
							}
							err = minioClient.RemoveObject(ctx.Context, bucket, object.Key, minio.RemoveObjectOptions{})
							if err != nil {
								return err
							}
							fmt.Printf("Removed %s\n", object.Key)
						}

						return nil
					},
				},
			},
		},
	},
}
