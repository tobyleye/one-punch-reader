package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

func indexPage(w http.ResponseWriter, r *http.Request) {

	html := `<html>
		<body>
			<h3>Welcome to your comic</h3>
			<a href="/page/1">
				<button>start reading here</button>
			</a>
		</body>
	</html>`

	w.Write([]byte(html))
}

func fetchComicsPagesFromLocal() []string {
	dir, err := os.ReadDir("./assets")
	if err != nil {
		log.Fatal("error reading comics directory")
	}
	comicsPages := []string{}

	for _, item := range dir {
		folderName := item.Name()

		subdir, err := os.ReadDir("./assets/" + folderName)
		if err == nil {
			for _, page := range subdir {
				comicPagePath := fmt.Sprintf("/assets/%s/%s", folderName, page.Name())
				comicsPages = append(comicsPages, comicPagePath)
			}
		}

	}

	return comicsPages

}

func getFilesFromFolder(driveService *drive.Service, folderId string) (*drive.FileList, error) {
	query := fmt.Sprintf("'%s' in parents", folderId)
	files, err := driveService.Files.List().Q(query).Fields("files(id, name)").PageSize(200).Do()
	if err != nil {
		return nil, err
	}
	return files, nil

}

func getFolderNo(folderName string) string {
	return strings.Split(folderName, "_c")[1]
}

func fetchComicsPagesFromGoogleDrive() []string {
	ctx := context.Background()
	driveService, err := drive.NewService(ctx, option.WithAPIKey(os.Getenv("GOOGLE_DRIVE_API_KEY")))
	if err != nil {
		fmt.Println("error connecting to drive service")
		log.Fatal(err)
	}

	fmt.Println("connected to drive successfully")

	const folderId string = "1V2N5gch1yRsFAwZ8ndtPTVoZSmYypUNT"

	chapters, err := getFilesFromFolder(driveService, folderId)

	if err != nil {
		fmt.Println("unable to retrieve files..", err.Error())
		return []string{}

	} else {

		sortFunction := func(i, j int) bool {
			i_folder := getFolderNo(chapters.Files[i].Name)
			j_folder := getFolderNo(chapters.Files[j].Name)

			return i_folder < j_folder

		}
		sort.Slice(chapters.Files, sortFunction)

		comicPages := []string{}

		totalChapters := len(chapters.Files)
		for i := 0; i < totalChapters; i++ {
			chapter := chapters.Files[i]
			fmt.Printf("reading chapter(%d/%d) %s", i+1, totalChapters, chapter.Name)
			pages, err := getFilesFromFolder(driveService, chapter.Id)

			if err == nil {
				sort.Slice(pages.Files, func(i, j int) bool {
					page_i := pages.Files[i].Name
					page_j := pages.Files[j].Name
					return page_i < page_j
				})

				for _, page := range pages.Files {
					downloadUrl := fmt.Sprintf("https://drive.usercontent.google.com/download?id=%s", page.Id)
					fmt.Println(downloadUrl)
					comicPages = append(comicPages, downloadUrl)
				}

			}
		}

		return comicPages

	}

}

func main() {

	var comicsPages []string

	fetchFromGoogleDrive := true

	if fetchFromGoogleDrive {
		comicsPages = fetchComicsPagesFromGoogleDrive()
	} else {
		comicsPages = fetchComicsPagesFromLocal()
	}

	sendPage := func(w http.ResponseWriter, r *http.Request) {
		rawPage := r.PathValue("page")
		page, err := strconv.Atoi(rawPage)
		if err != nil {
			http.Redirect(w, r, "/page/1", http.StatusSeeOther)
			return
		}

		lastPage := len(comicsPages)

		if page > lastPage {
			redirectUrl := fmt.Sprintf("/page/%d", lastPage)
			http.Redirect(w, r, redirectUrl, http.StatusTemporaryRedirect)
			return
		}

		if page <= 0 {
			http.Redirect(w, r, "/page/1", http.StatusTemporaryRedirect)
			return
		}

		html := `<html>
			<body>
				<div>
					<img src="%s" style="" />
					</div>
						
					<a href="/page/%d">
					<button>
						back
					</button>
					</a>
					<a href="/page/%d">
						<button>next</button>
					</a>
					
			</body>
		</html>`

		pageComic := comicsPages[page-1]
		nextPage := page + 1
		prevPage := page - 1

		html = fmt.Sprintf(html, pageComic, prevPage, nextPage)

		response := []byte(html)
		w.Write(response)
	}

	http.HandleFunc("GET /page/{page}", sendPage)
	http.Handle("/assets/",
		http.StripPrefix("/assets", http.FileServer(http.Dir("./assets"))))
	http.HandleFunc("/", indexPage)

	fmt.Print("starting server...")
	http.ListenAndServe(":8080", nil)
}
