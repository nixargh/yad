package yad

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"
)

const baseURL = "https://cloud-api.yandex.net/v1/disk"

type Client struct {
	oauthToken string
	baseURl    string
	appFolder  bool
	client     *http.Client
}

func NewClient(oauthToken string, timeout time.Duration, appFolder bool) (*Client, error) {
	if timeout == 0 {
		return nil, errors.New("timeout can't be zero")
	}

	return &Client{
		oauthToken: oauthToken,
		baseURl:    baseURL,
		appFolder:  appFolder,
		client: &http.Client{
			Timeout:       timeout,
			Transport:     transport,
			CheckRedirect: checkRedirect,
		},
	}, nil
}

func (c *Client) sendRequest(req *http.Request, data interface{}) error {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "OAuth "+c.oauthToken)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		errorResponse := ErrorResponse{}
		if err = json.NewDecoder(resp.Body).Decode(&errorResponse); err == nil {
			return fmt.Errorf("%sstatus code: %d\n", errorResponse.Info(), resp.StatusCode)
		}

		return fmt.Errorf("unknown error, status code: %d\n", resp.StatusCode)
	}

	json.NewDecoder(resp.Body).Decode(&data)

	return nil
}

func (c *Client) sendRequestNew(req *http.Request, data interface{}) (int, error) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "OAuth "+c.oauthToken)

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// Debug
	fmt.Printf("Status code: %d\n", resp.StatusCode)

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		errorResponse := ErrorResponse{}

		err = json.NewDecoder(resp.Body).Decode(&errorResponse)

		if err != nil {
			return resp.StatusCode, fmt.Errorf("Can't decode HTTP body")
		}

		return resp.StatusCode, fmt.Errorf(errorResponse.Description)
	}

	json.NewDecoder(resp.Body).Decode(&data)

	return resp.StatusCode, nil
}

func (c *Client) OperationStatus(ctx context.Context, url string) (*SuccessResponse, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)

	var code int
	successResponse := SuccessResponse{}
	if code, err = c.sendRequestNew(req, &successResponse); err != nil {
		return nil, err
	}

	if code != 200 {
		return nil, fmt.Errorf("HTTP code is not 200")
	}

	//	response := successResponse

	return &successResponse, nil

}

func (c *Client) Mkdir(ctx context.Context, path string, ignoreExistance bool) (*SuccessResponse, error) {
	if path == "" {
		return nil, errors.New("paths can't be empty")
	}

	urlSuffix := ""
	if c.appFolder == true {
		urlSuffix = "app:/"
	}

	req, err := http.NewRequest("PUT", c.baseURl+"/resources", nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Set("path", urlSuffix+path)
	req.URL.RawQuery = q.Encode()

	req = req.WithContext(ctx)

	var code int
	successResponse := SuccessResponse{}
	code, err = c.sendRequestNew(req, &successResponse)

	if err != nil {
		// Directory exists
		if code == 409 && ignoreExistance {
			return &successResponse, nil
		}

		return nil, err
	}

	return &successResponse, nil
}

func (c *Client) Upload(ctx context.Context, path string, overwrite bool) (*SuccessResponse, error) {
	if path == "" {
		return nil, errors.New("path can't be empty")
	}

	var overwriteString string

	if overwrite {
		overwriteString = "true"
	} else {
		overwriteString = "false"
	}

	urlSuffix := ""
	if c.appFolder == true {
		urlSuffix = "app:/"
	}

	req, err := http.NewRequest("GET", c.baseURl+"/resources/upload", nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Set("path", urlSuffix+path)
	q.Set("overwrite", overwriteString)
	req.URL.RawQuery = q.Encode()

	req = req.WithContext(ctx)

	successResponse := SuccessResponse{}

	_, err = c.sendRequestNew(req, &successResponse)
	if err != nil {
		return nil, err
	}

	return &successResponse, nil
}

func (c *Client) UploadByURL(ctx context.Context, path string, url string) (*http.Response, error) {
	if path == "" || url == "" {
		return nil, errors.New("path or url can't be empty")
	}

	data, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PUT", url, data)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	req.URL.RawQuery = q.Encode()

	client := &http.Client{}
	resp, err := client.Do(req)
	defer resp.Body.Close()

	/* body := Body{}

	err := json.NewDecoder(resp.Body).Decode(&body)
	if err != nil {
		return resp, err
	}*/

	switch resp.StatusCode {
	case 201:
		return resp, nil
	case 202:
		/*
			status, err := c.OperationStatus(ctx, data.OperationId)
			if err != nil {
				return nil, err
			}
			fmt.Printf("Operation status: %+v\n", resp.Body)
		*/
		// TODO
		fmt.Printf("TO DO: write operation status check loop")
	}

	return resp, errors.New("Unknown response status")
}

/* TODO
Everything under that comment aren't tested or know as broken
*/

func (c *Client) GetDiskInfo(ctx context.Context) (*Disk, error) {
	req, err := http.NewRequest("GET", c.baseURl, nil)
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)

	disk := Disk{}

	if err = c.sendRequest(req, &disk); err != nil {
		return nil, err
	}

	return &disk, nil

}

func (c *Client) GetFiles(ctx context.Context, limit int) (*FilesResourceList, error) {
	urlSuffix := ""
	if c.appFolder == true {
		urlSuffix = "?path=app:/"
	}

	req, err := http.NewRequest("GET", c.baseURl+"/resources/files"+urlSuffix, nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Set("limit", strconv.Itoa(limit))
	req.URL.RawQuery = q.Encode()

	req = req.WithContext(ctx)

	filesResourceList := FilesResourceList{}

	if err = c.sendRequest(req, &filesResourceList); err != nil {
		return nil, err
	}

	return &filesResourceList, nil
}

func (c *Client) Delete(ctx context.Context, path string, permanently bool) (*SuccessResponse, error) {
	if path == "" {
		return nil, errors.New("path can't be empty")
	}

	req, err := http.NewRequest("DELETE", c.baseURl+"/resources", nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Set("path", path)
	q.Set("permanently", strconv.FormatBool(permanently))
	req.URL.RawQuery = q.Encode()

	req = req.WithContext(ctx)

	successResponse := SuccessResponse{}

	if err = c.sendRequest(req, &successResponse); err != nil {
		return nil, err
	}

	return &successResponse, nil
}

func (c *Client) Download(ctx context.Context, path string) (*SuccessResponse, error) {
	if path == "" {
		return nil, errors.New("path can't be empty")
	}

	req, err := http.NewRequest("GET", c.baseURl+"/resources/download", nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Set("path", path)
	req.URL.RawQuery = q.Encode()

	req = req.WithContext(ctx)

	successResponse := SuccessResponse{}

	if err = c.sendRequest(req, &successResponse); err != nil {
		return nil, err
	}

	return &successResponse, nil
}

func (c *Client) Publish(ctx context.Context, path string) (*SuccessResponse, error) {
	if path == "" {
		return nil, errors.New("path can't be empty")
	}

	req, err := http.NewRequest("PUT", c.baseURl+"/resources/publish", nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Set("path", path)
	req.URL.RawQuery = q.Encode()

	req = req.WithContext(ctx)

	successResponse := SuccessResponse{}

	if err = c.sendRequest(req, &successResponse); err != nil {
		return nil, err
	}

	return &successResponse, nil
}

func (c *Client) Unpublish(ctx context.Context, path string) (*SuccessResponse, error) {
	if path == "" {
		return nil, errors.New("path can't be empty")
	}

	req, err := http.NewRequest("PUT", c.baseURl+"/resources/unpublish", nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Set("path", path)
	req.URL.RawQuery = q.Encode()

	req = req.WithContext(ctx)

	successResponse := SuccessResponse{}

	if err = c.sendRequest(req, &successResponse); err != nil {
		return nil, err
	}

	return &successResponse, nil
}

func (c *Client) GetPublicFiles(ctx context.Context, limit int) (*FilesResourceList, error) {
	req, err := http.NewRequest("GET", c.baseURl+"/resources/public", nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Set("limit", strconv.Itoa(limit))
	req.URL.RawQuery = q.Encode()

	req = req.WithContext(ctx)

	filesResourceList := FilesResourceList{}

	if err = c.sendRequest(req, &filesResourceList); err != nil {
		return nil, err
	}

	return &filesResourceList, nil
}

func (c *Client) Move(ctx context.Context, from string, path string) (*SuccessResponse, error) {
	if path == "" || from == "" {
		return nil, errors.New("paths can't be empty")
	}

	req, err := http.NewRequest("POST", c.baseURl+"/resources/move", nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Set("from", from)
	q.Set("path", path)
	req.URL.RawQuery = q.Encode()

	req = req.WithContext(ctx)

	successResponse := SuccessResponse{}

	if err = c.sendRequest(req, &successResponse); err != nil {
		return nil, err
	}

	return &successResponse, nil
}

func (c *Client) Copy(ctx context.Context, from string, path string) (*SuccessResponse, error) {
	if path == "" || from == "" {
		return nil, errors.New("paths can't be empty")
	}

	req, err := http.NewRequest("POST", c.baseURl+"/resources/copy", nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Set("from", from)
	q.Set("path", path)
	req.URL.RawQuery = q.Encode()

	req = req.WithContext(ctx)

	successResponse := SuccessResponse{}

	if err = c.sendRequest(req, &successResponse); err != nil {
		return nil, err
	}

	return &successResponse, nil
}

func (c *Client) GetTrash(ctx context.Context, path string, limit int) (*TrashResourceList, error) {
	if path == "" {
		return nil, errors.New("path can't be empty")
	}

	req, err := http.NewRequest("GET", c.baseURl+"/trash/resources", nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Set("limit", strconv.Itoa(limit))
	q.Set("path", path)
	req.URL.RawQuery = q.Encode()

	req = req.WithContext(ctx)

	trashResourceList := TrashResourceList{}

	if err = c.sendRequest(req, &trashResourceList); err != nil {
		return nil, err
	}

	return &trashResourceList, nil
}

// only full trash path, trash:/ works
func (c *Client) ClearTrash(ctx context.Context, path string) (*SuccessResponse, error) {
	if path == "" {
		return nil, errors.New("path can't be empty")
	}

	req, err := http.NewRequest("DELETE", c.baseURl+"/trash/resources", nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Set("path", path)
	req.URL.RawQuery = q.Encode()

	req = req.WithContext(ctx)

	successResponse := SuccessResponse{}

	if err = c.sendRequest(req, &successResponse); err != nil {
		return nil, err
	}

	return &successResponse, nil
}

// only full trash path, trash:/ doesn't work
func (c *Client) RestoreTrash(ctx context.Context, path string) (*SuccessResponse, error) {
	if path == "" {
		return nil, errors.New("path can't be empty")
	}

	req, err := http.NewRequest("PUT", c.baseURl+"/trash/resources/restore", nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Set("path", path)
	req.URL.RawQuery = q.Encode()

	req = req.WithContext(ctx)

	successResponse := SuccessResponse{}

	if err = c.sendRequest(req, &successResponse); err != nil {
		return nil, err
	}

	return &successResponse, nil
}
