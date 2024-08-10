package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/schollz/progressbar/v3"
)

// A list of site categories used to help organise models.
var CATEGORIES = []string{
	"character",
	"style",
	"celebrity",
	"concept",
	"clothing",
	"base model",
	"poses",
	"background",
	"tool",
	"buildings",
	"vehicle",
	"objects",
	"animal",
	"action",
	"asset",
}

// GetCategory guesses a category for each model based on its tags
func GetCategory(tags *[]string) string {
	for _, tag := range *tags {
		for _, category := range CATEGORIES {
			if strings.ToLower(tag) == category {
				return tag
			}
		}
	}
	return "misc"
}

// This used to push updates to the progress bar
type IntFunc func(int)

type CivitaiFile struct {
	SizeKB  float32
	Name    string `json:"name"`
	Type    string `json:"type"`
	Primary bool   `json:"primary"`
	URL     string `json:"downloadURL"`
}

type CivitaiImage struct {
	URL string `json:"url"`
}

type CivitaiModel struct {
	Id       int
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Tags     []string `json:"tags"`
	Category string
}

type CivitaiModelVersion struct {
	Id      int            `json:"id"`
	Name    string         `json:"name"`
	ModelId int            `json:"modelId"`
	Type    string         `json:"type"`
	Files   []CivitaiFile  `json:"files"`
	Images  []CivitaiImage `json:"images"`
}

func (c *CivitaiModelVersion) PrimaryFile() *CivitaiFile {
	for _, f := range c.Files {
		if f.Primary {
			return &f
		}

	}
	return nil
}

type CivitaiClient struct {
	host    string
	api_key *string
	client  http.Client
}

func NewCivitaiClient(api_key *string) CivitaiClient {
	return CivitaiClient{
		host:    "https://civitai.com/api/v1",
		api_key: api_key,
		client:  http.Client{},
	}
}

func (c *CivitaiClient) DownloadModelVersion(model_version *CivitaiModelVersion, directory string) error {
	primary_file := model_version.PrimaryFile()
	if primary_file == nil {
		return fmt.Errorf("%s has no primary file.", model_version.Name)
	}

	model_path := filepath.Join(directory, primary_file.Name)

	err := c.DownloadFile(primary_file, model_path)
	if err != nil {
		return err
	}

	slog.Info("Downloading Images")
	for _, image := range model_version.Images {
		image_name := filepath.Base(image.URL)
		image_path := filepath.Join(directory, image_name)
		err = c.DownloadImage(&image, image_path)
		if err != nil {
			return err
		}
	}
	return nil

}

func (c *CivitaiClient) DownloadImage(image *CivitaiImage, path string) error {
	return c.Download(image.URL, path, nil)
}

func (c *CivitaiClient) DownloadFile(file *CivitaiFile, path string) error {
	url := file.URL
	total_bytes := int(file.SizeKB * 1024)
	bar := progressbar.New(total_bytes)
	defer bar.Close()
	bar.Describe("Downloading: " + file.Name)

	cb := func(a int) { bar.Add(a) }

	return c.Download(url, path, cb)

}

func (c *CivitaiClient) Download(url string, filename string, progress IntFunc) error {
	slog.Debug("Downloading: " + url)
	response, err := c.Get(url, nil)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	file, err := os.Create(filename)
	if err != nil {
		return err
	}

	defer file.Close()
	chunk_size := 1024
	buffer := make([]byte, chunk_size)

	for {
		n, err := response.Body.Read(buffer)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			break
		}

		_, err = file.Write(buffer[:n])
		if err != nil {

			return err
		}
		if progress != nil {
			progress(n)
		}
	}
	return nil

}
func (c *CivitaiClient) Do(method string, url string, headers *map[string]string) (*http.Response, error) {
	req, err := http.NewRequest(method, url, nil)

	if headers != nil {
		for key, value := range *headers {
			req.Header.Add(key, value)
		}
	}

	if c.api_key != nil {
		req.Header.Add("Authorization", "Bearer "+*c.api_key)
	}

	if err != nil {
		return nil, err
	}

	slog.Debug(fmt.Sprintf("retrieving data from url: %s", url))
	response, err := c.client.Do(req)

	if err != nil {
		return nil, err
	}

	code := response.StatusCode
	if code == 200 {
		return response, nil
	} else if code == 401 {
		return nil, fmt.Errorf("HTTP Error: 401: Unauthorized Request, Check your API KEY.")
	} else if code >= 400 && code < 500 {
		return nil, fmt.Errorf("HTTP Error: %d: Bad Request. Civitai may be down.", code)
	} else if code >= 500 && code < 600 {
		return nil, fmt.Errorf("HTTP Error: %d: Service Unavailable. Civitai may be down or conducting maintenance.", code)
	} else {
		return nil, fmt.Errorf("HTTP Error: %d", code)
	}
}

func (c *CivitaiClient) Get(url string, headers *map[string]string) (*http.Response, error) {
	return c.Do("GET", url, headers)
}

func (c *CivitaiClient) Head(url string, headers *map[string]string) (*http.Response, error) {
	return c.Do("HEAD", url, headers)
}

func (c *CivitaiClient) GetJson(url string) (*http.Response, error) {
	headers := make(map[string]string)
	headers["Content-Type"] = "application/json"
	return c.Get(url, &headers)
}

func (c *CivitaiClient) GetModel(model_id int) (*CivitaiModel, error) {
	slog.Debug(fmt.Sprintf("Getting Model from ID: %d", model_id))
	url := fmt.Sprintf("%s/models/%d", c.host, model_id)
	response, err := c.GetJson(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var model CivitaiModel
	data_as_bytes, err := io.ReadAll(response.Body)

	if err != nil {
		return nil, err
	}

	if json.Unmarshal(data_as_bytes, &model) != nil {
		return nil, err
	}

	model.Category = GetCategory(&model.Tags)

	return &model, nil

}
func (c *CivitaiClient) GetModelVersion(version_id int) (*CivitaiModelVersion, error) {
	slog.Debug(fmt.Sprintf("Getting Model Version from ID: %d", version_id))
	url := fmt.Sprintf("%s/model-versions/%d", c.host, version_id)

	response, err := c.GetJson(url)

	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		return nil, fmt.Errorf("Request failed with error code: %d", response.StatusCode)
	}

	var model_version CivitaiModelVersion

	data_as_bytes, err := io.ReadAll(response.Body)

	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data_as_bytes, &model_version)
	if err != nil {
		return nil, err
	}

	return &model_version, nil
}
