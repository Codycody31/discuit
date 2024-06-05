package storage

import (
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/discuitnet/discuit/config"
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
						imagesToUpload := []string{}

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

						// Get all images
						err = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
							if !d.IsDir() {
								imagesToUpload = append(imagesToUpload, path)
							}
							return nil
						})
						if err != nil {
							return err
						}
						fmt.Printf("Found %d images to upload\n", len(imagesToUpload))

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

						for _, image := range imagesToUpload {
							objectName := strings.Split(image, p+"/")[1]
							// filePath := image
							// contentType := "image/jpeg"

							// Check if the image already exists in the bucket
							stat, err := minioClient.StatObject(ctx.Context, bucket, objectName, minio.StatObjectOptions{})
							if err != nil {
								if err.Error() == "The specified key does not exist." {
									// Ignore
								} else {
									return fmt.Errorf("Failed to check if image %s exists in the bucket: %v", objectName, err)
								}
							}
							if stat.Size > 0 {
								fmt.Printf("Image %s already exists in the bucket\n", objectName)
								continue
							}

							// Upload the test file with FPutObject
							// info, err := minioClient.FPutObject(ctx.Context, bucket, objectName, filePath, minio.PutObjectOptions{ContentType: contentType})
							// if err != nil {
							// 	return fmt.Errorf("Failed to upload %s: %v", objectName, err)
							// }

							// log.Printf("Successfully uploaded %s of size %d\n", objectName, info.Size)

							if ctx.Bool("clean") {
								err = os.Remove(image)
								if err != nil {
									return err
								}
								fmt.Printf("Removed %s\n", image)
							}

							// Extract the id from the image path and add it to the array (/36/5/{last part of id})
							idParts := strings.Split(image, "/")
							id := idParts[len(idParts)-3] + idParts[len(idParts)-2]

							// The last part is the id but we need to strip the extension and anything including and after the underscore
							id = id + strings.Split(strings.Split(idParts[len(idParts)-1], "_")[0], ".")[0]

							// Add id to the array if it's not already in there
							if id != "" && !utils.StringInSlice(id, imageIDs) {
								// TODO: Convert to string equivalent of uid.ID
								imageIDs = append(imageIDs, id)
							}

							// if len(imageIDs) > 1000 {
							// 	tx, err := db.BeginTx(ctx.Context, nil)
							// 	// Build where clause

							// 	// TODO: Check if length of array is greater than 1000, and then call DB to change the store name for the images
							// 	query, args := msql.BuildUpdateQuery("images", []msql.ColumnValue{{Name: "store", Value: "s3"}}, fmt.Sprintf("id IN %s", msql.BuildInClause(imageIDs)))
							// 	fmt.Println(query)
							// 	fmt.Println(args)

							// 	if _, err = tx.ExecContext(ctx.Context, query, args...); err != nil {
							// 		return err
							// 	}

							// 	if err = tx.Commit(); err != nil {
							// 		return err
							// 	}

							// 	fmt.Println("Successfully changed the store name for the images")
							// }
						}

						if len(imageIDs) > 0 {
							tx, err := db.BeginTx(ctx.Context, nil)
							// Build where clause

							// TODO: Check if length of array is greater than 1000, and then call DB to change the store name for the images
							query, args := msql.BuildUpdateQuery("images", []msql.ColumnValue{{Name: "store_name", Value: "s3"}}, fmt.Sprintf("WHERE id IN %s", msql.BuildInClause(imageIDs)))
							fmt.Println(query)
							fmt.Println(args)
							return nil

							if _, err = tx.ExecContext(ctx.Context, query, args...); err != nil {
								return err
							}

							if err = tx.Commit(); err != nil {
								return err
							}

							fmt.Println("Successfully changed the store name for the images")
						}

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
