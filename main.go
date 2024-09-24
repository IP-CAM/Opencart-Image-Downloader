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
    "sync"

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

    progressBar := widget.NewProgressBar()
    statusLabel := widget.NewLabel("Status: Idle")

    mainImageText := widget.NewMultiLineEntry()
    mainImageText.SetPlaceHolder("New main_image content will appear here")

    imageCacheText := widget.NewMultiLineEntry()
    imageCacheText.SetPlaceHolder("New image_cache content will appear here")

    downloadButton := widget.NewButton("Download Images", func() {
        go func() {
            spreadsheetURL := urlEntry.Text
            if spreadsheetURL == "" {
                showError(myWindow, errors.New("Please enter a URL"))
                return
            }

            statusLabel.SetText("Status: Fetching CSV data...")
            csvURL, err := getCSVURL(spreadsheetURL)
            if err != nil {
                showError(myWindow, err)
                statusLabel.SetText("Status: Idle")
                return
            }

            records, err := fetchCSV(csvURL)
            if err != nil {
                showError(myWindow, err)
                statusLabel.SetText("Status: Idle")
                return
            }

            statusLabel.SetText("Status: Checking existing images...")
            if dirExists("products") {
                dialog.ShowConfirm("Directory Exists",
                    `"products" directory already exists. Do you want to delete it and proceed?`,
                    func(b bool) {
                        if b {
                            err := os.RemoveAll("products")
                            if err != nil {
                                showError(myWindow, fmt.Errorf("Failed to delete 'products' directory: %v", err))
                                return
                            }
                            continueProcessing(records, statusLabel, progressBar, mainImageText, imageCacheText, myWindow)
                        } else {
                            statusLabel.SetText("Status: Operation Aborted")
                            showError(myWindow, fmt.Errorf("'products' directory exists, aborting ..."))
                            return
                        }
                    }, myWindow)
            } else {
                continueProcessing(records, statusLabel, progressBar, mainImageText, imageCacheText, myWindow)
            }
        }()
    })

    content := container.NewVBox(
        urlEntry,
        downloadButton,
        progressBar,
        statusLabel,
        widget.NewLabel("New main_image Data:"),
        mainImageText,
        widget.NewLabel("New image_cache Data:"),
        imageCacheText,
    )

    myWindow.SetContent(content)
    myWindow.Resize(fyne.NewSize(800, 600))
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

func processRecords(records [][]string, progressBar *widget.ProgressBar, statusLabel *widget.Label, mainImageText, imageCacheText *widget.Entry) error {
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

    totalRows := len(records) - 1
    progressBar.Max = float64(totalRows)
    progressBar.SetValue(0)

    var mainImageData []string
    var imageCacheData []string

    var mu sync.Mutex // To synchronize access to UI elements
    for rowIndex, row := range records[1:] {
        fyne.CurrentApp().SendNotification(&fyne.Notification{
            Title:   "Processing",
            Content: fmt.Sprintf("Processing row %d/%d", rowIndex+1, totalRows),
        })

        mainImageURL := row[headerMap["main_image"]]
        imageCacheURLs := row[headerMap["image_cache"]]
        brandSEOURL := row[headerMap["brand_seo_url"]]
        seoURL := row[headerMap["seo_url"]]

        var newMainImagePath string
        if mainImageURL != "" {
            statusLabel.SetText(fmt.Sprintf("Status: Downloading main_image (%d/%d)", rowIndex+1, totalRows))
            newPath, err := downloadAndSaveImage(mainImageURL, brandSEOURL, seoURL, fmt.Sprintf("main_image_%d", rowIndex))
            if err != nil {
                fmt.Printf("Error downloading main_image for row %d: %v\n", rowIndex+2, err)
            }
            newMainImagePath = newPath
        }

        var newImageCachePaths []string
        if imageCacheURLs != "" {
            statusLabel.SetText(fmt.Sprintf("Status: Downloading image_cache (%d/%d)", rowIndex+1, totalRows))
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
                    newPath, err := downloadAndSaveImage(imgURL, brandSEOURL, seoURL, fmt.Sprintf("image_cache_%d_%d", rowIndex, i))
                    if err != nil {
                        fmt.Printf("Error downloading image_cache for row %d: %v\n", rowIndex+2, err)
                    }
                    newImageCachePaths = append(newImageCachePaths, newPath)
                }
            }
        }

        // Update the main_image and image_cache data
        mu.Lock()
        if newMainImagePath != "" {
            mainImageData = append(mainImageData, newMainImagePath)
        } else {
            mainImageData = append(mainImageData, "")
        }

        if len(newImageCachePaths) > 0 {
            imageCacheData = append(imageCacheData, strings.Join(newImageCachePaths, "|"))
        } else {
            imageCacheData = append(imageCacheData, "")
        }

        // Update progress bar and status label
        progressBar.SetValue(float64(rowIndex + 1))
        mu.Unlock()
    }

    // Update the text boxes with new data
//    fyne.CurrentApp().RunOnMain(func() {
        mainImageText.SetText(strings.Join(mainImageData, "\n"))
        imageCacheText.SetText(strings.Join(imageCacheData, "\n"))
//    })

    return nil
}

func continueProcessing(records [][]string, statusLabel *widget.Label, progressBar *widget.ProgressBar, mainImageText, imageCacheText *widget.Entry, myWindow fyne.Window) {
    statusLabel.SetText("Status: Processing records...")
    err := processRecords(records, progressBar, statusLabel, mainImageText, imageCacheText)
    if err != nil {
        showError(myWindow, err)
        statusLabel.SetText("Status: Idle")
        return
    }

    statusLabel.SetText("Status: Completed")
    showInfo(myWindow, "Images downloaded successfully")
}

func downloadAndSaveImage(imageURL, brandSEOURL, seoURL, imageType string) (string, error) {
    baseDir := "products"
    brandDir := filepath.Join(baseDir, brandSEOURL)
    err := os.MkdirAll(brandDir, os.ModePerm)
    if err != nil {
        return "", err
    }

    ext := filepath.Ext(imageURL)
    if ext == "" || len(ext) > 5 {
        ext = ".jpg"
    }

    filename := fmt.Sprintf("%s_%s%s", seoURL, imageType, ext)
    filePath := filepath.Join(brandDir, filename)
    relativePath := filepath.ToSlash(filePath) // For consistent path separators

    if _, err := os.Stat(filePath); err == nil {
        // File already exists
        return relativePath, nil
    }

    resp, err := http.Get(imageURL)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return "", fmt.Errorf("Failed to download image: %s", resp.Status)
    }

    out, err := os.Create(filePath)
    if err != nil {
        return "", err
    }
    defer out.Close()

    _, err = io.Copy(out, resp.Body)
    if err != nil {
        return "", err
    }

    fmt.Printf("Downloaded image: %s\n", filePath)
    return relativePath, nil
}

func showError(win fyne.Window, err error) {
    fyne.CurrentApp().SendNotification(&fyne.Notification{
        Title:   "Error",
        Content: err.Error(),
    })
    dialog.ShowError(err, win)
}

func showInfo(win fyne.Window, message string) {
    fyne.CurrentApp().SendNotification(&fyne.Notification{
        Title:   "Success",
        Content: message,
    })
    dialog.ShowInformation("Success", message, win)
}

func dirExists(dir string) bool {
    info, err := os.Stat(dir)
    if os.IsNotExist(err) {
        return false
    }
    return info.IsDir()
}

