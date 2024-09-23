package main

import (
    "encoding/csv"
    "errors"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "os"
    "path/filepath"
    "strings"

    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/app"
    "fyne.io/fyne/v2/container"
    "fyne.io/fyne/v2/dialog"
    "fyne.io/fyne/v2/widget"
)

func main() {
    myApp := app.New()
    myWindow := myApp.NewWindow("Google Spreadsheet Image Downloader")

    urlEntry := widget.NewEntry()
    urlEntry.SetPlaceHolder("Enter Google Spreadsheet URL")
    downloadButton := widget.NewButton("Download Images", func() {
        go func() {
            spreadsheetURL := urlEntry.Text
            if spreadsheetURL == "" {
                showError(myWindow, errors.New("Please enter a URL"))
                return
            }

            csvURL, err := getCSVURL(spreadsheetURL)
            if err != nil {
                showError(myWindow, err)
                return
            }

            records, err := fetchCSV(csvURL)
            if err != nil {
                showError(myWindow, err)
                return
            }

            err = processRecords(records)
            if err != nil {
                showError(myWindow, err)
                return
            }

            showInfo(myWindow, "Images downloaded successfully")
        }()
    })

    content := container.NewVBox(
        urlEntry,
        downloadButton,
    )

    myWindow.SetContent(content)
    myWindow.Resize(fyne.NewSize(600, 100))
    myWindow.ShowAndRun()
}

func getCSVURL(spreadsheetURL string) (string, error) {
    u, err := url.Parse(spreadsheetURL)
    if err != nil {
        return "", err
    }

    parts := strings.Split(u.Path, "/")
    var spreadsheetID string
    for i, part := range parts {
        if part == "d" && i+1 < len(parts) {
            spreadsheetID = parts[i+1]
            break
        }
    }
    if spreadsheetID == "" {
        return "", errors.New("Invalid Google Spreadsheet URL")
    }

    q := u.Query()
    gid := q.Get("gid")
    if gid == "" {
        if u.Fragment != "" {
            fragParts := strings.Split(u.Fragment, "=")
            if len(fragParts) == 2 && fragParts[0] == "gid" {
                gid = fragParts[1]
            }
        }
        if gid == "" {
            gid = "0"
        }
    }

    csvURL := fmt.Sprintf("https://docs.google.com/spreadsheets/d/%s/export?format=csv&gid=%s", spreadsheetID, gid)

    return csvURL, nil
}

func fetchCSV(csvURL string) ([][]string, error) {
    resp, err := http.Get(csvURL)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("Failed to fetch CSV data: %s", resp.Status)
    }

    reader := csv.NewReader(resp.Body)
    records, err := reader.ReadAll()
    if err != nil {
        return nil, err
    }

    return records, nil
}

func processRecords(records [][]string) error {
    if len(records) < 2 {
        return errors.New("No data in CSV")
    }

    headers := records[0]
    headerMap := make(map[string]int)
    for i, h := range headers {
        headerMap[h] = i
    }

    requiredColumns := []string{"main_image", "image_cache", "brand_seo_url", "seo_url"}
    for _, col := range requiredColumns {
        if _, ok := headerMap[col]; !ok {
            return fmt.Errorf("Missing required column: %s", col)
        }
    }

    for rowIndex, row := range records[1:] {
        mainImageURL := row[headerMap["main_image"]]
        imageCacheURLs := row[headerMap["image_cache"]]
        brandSEOURL := row[headerMap["brand_seo_url"]]
        seoURL := row[headerMap["seo_url"]]

        if mainImageURL != "" {
            err := downloadAndSaveImage(mainImageURL, brandSEOURL, seoURL, fmt.Sprintf("main_image_%d", rowIndex))
            if err != nil {
                fmt.Printf("Error downloading main_image for row %d: %v\n", rowIndex+2, err)
            }
        }

        if imageCacheURLs != "" {
            var urls []string
            if strings.Contains(imageCacheURLs, "|") {
                urls = strings.Split(imageCacheURLs, "|")
            } else if strings.Contains(imageCacheURLs, ",") {
                urls = strings.Split(imageCacheURLs, ",")
            } else {
                urls = []string{imageCacheURLs}
            }

            for i, imgURL := range urls {
                imgURL = strings.TrimSpace(imgURL)
                if imgURL != "" {
                    err := downloadAndSaveImage(imgURL, brandSEOURL, seoURL, fmt.Sprintf("image_cache_%d_%d", rowIndex, i))
                    if err != nil {
                        fmt.Printf("Error downloading image_cache for row %d: %v\n", rowIndex+2, err)
                    }
                }
            }
        }
    }

    return nil
}

func downloadAndSaveImage(imageURL, brandSEOURL, seoURL, imageType string) error {
    baseDir := "products"
    brandDir := filepath.Join(baseDir, brandSEOURL)
    err := os.MkdirAll(brandDir, os.ModePerm)
    if err != nil {
        return err
    }

    ext := filepath.Ext(imageURL)
    if ext == "" {
        ext = ".jpg"
    }

    filename := fmt.Sprintf("%s_%s%s", seoURL, imageType, ext)
    filePath := filepath.Join(brandDir, filename)

    if _, err := os.Stat(filePath); err == nil {
        return nil
    }

    resp, err := http.Get(imageURL)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("Failed to download image: %s", resp.Status)
    }

    out, err := os.Create(filePath)
    if err != nil {
        return err
    }
    defer out.Close()

    _, err = io.Copy(out, resp.Body)
    if err != nil {
        return err
    }

    fmt.Printf("Downloaded image: %s\n", filePath)
    return nil
}

func showError(win fyne.Window, err error) {
    dialog.ShowError(err, win)
}

func showInfo(win fyne.Window, message string) {
    dialog.ShowInformation("Success", message, win)
}

