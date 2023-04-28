package yad

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	//	"github.com/pkg/profile"
	log "github.com/sirupsen/logrus"
)

var alog *log.Entry

type API struct {
	client *Client
}

func NewAPI(oauthToken string, timeout time.Duration, appFolder bool, clog *log.Entry) *API {
	hiddenToken := fmt.Sprintf("%s...", oauthToken[0:5])

	alog = clog.WithFields(log.Fields{
		"oauthToken": hiddenToken,
		"timeout":    timeout,
		"appFolder":  appFolder,
		"logger":     "yad.api",
	})

	alog.Info("Creating new YaD client.")

	c, err := NewClient(oauthToken, timeout, appFolder)
	if err != nil {
		alog.WithFields(log.Fields{"error": err}).Fatal("Can't create YaD Client.")
	}

	return &API{
		client: c,
	}
}

func (api *API) Upload(srcPath string, dstPath string, overwrite bool) bool {
	alog.WithFields(log.Fields{
		"srcPath":   srcPath,
		"dstPath":   dstPath,
		"overwrite": overwrite,
	}).Info("Uploading file(s).")

	api.createPath(dstPath, true)

	ctx := context.Background()

	res, err := api.client.Upload(ctx, dstPath, overwrite)
	if err != nil {
		alog.WithFields(log.Fields{
			"file":  dstPath,
			"error": err,
		}).Error("Can't get upload URL.")

		return false
	}
	alog.Info(res.Href)

	r, e := api.client.UploadByURL(ctx, srcPath, res.Href)
	if e != nil {
		alog.WithFields(log.Fields{
			"file":  srcPath,
			"url":   res.Href,
			"code":  r,
			"error": e,
		}).Error("Upload failed.")

		return false
	}
	alog.Info(r)

	return true
}

func (api *API) UploadChannelling(inputChan chan [2]string, errorChan chan error, overwrite bool) {

	alog.WithFields(log.Fields{
		"overwrite": overwrite,
	}).Info("Starting channelling upload.")

	finished := false
	for !finished {
		select {
		case file, opened := <-inputChan:
			if opened {
				srcPath := file[0]
				dstPath := file[1]

				if api.createPath(dstPath, true) == false {
					errorChan <- fmt.Errorf("Can't create dir for: %s", dstPath)
					continue
				}

				ctx := context.Background()

				res, err := api.client.Upload(ctx, dstPath, overwrite)
				if err != nil {
					alog.WithFields(log.Fields{
						"file":  dstPath,
						"error": err,
					}).Error("Can't get upload URL.")
					errorChan <- fmt.Errorf("Can't get upload URL for: %s", dstPath)
					continue
				}

				r, e := api.client.UploadByURL(ctx, srcPath, res.Href)
				if e != nil {
					alog.WithFields(log.Fields{
						"file":  srcPath,
						"url":   res.Href,
						"code":  r,
						"error": e,
					}).Error("Upload failed.")
					errorChan <- fmt.Errorf("Upload failed for: %s", srcPath)
					continue
				}
			} else {
				alog.Info("Input channel is closed.")
				finished = true
			}

		default:
			time.Sleep(100 * time.Millisecond)
		}
	}

	close(errorChan)
}

func (api *API) createPath(path string, ignoreExistance bool) bool {
	dir := filepath.Dir(path)

	if dir == "." {
		return true
	}

	alog.WithFields(log.Fields{
		"directory": dir,
	}).Info("Creating directory.")

	ctx := context.Background()
	res, err := api.client.Mkdir(ctx, dir, ignoreExistance)
	if err != nil {
		alog.WithFields(log.Fields{
			"directory": dir,
			"code":      res,
			"error":     err,
		}).Error("Failed to create directory.")

		return false
	}

	return true
}
