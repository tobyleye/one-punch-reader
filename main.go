package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

const PageState string = "./page"

func indexPage(w http.ResponseWriter, r *http.Request) {
	currentPage := getPage()
	fmt.Println("current page is ", currentPage)
	t := template.Must(template.New("index.html").ParseFiles("./templates/index.html"))
	t.Execute(w, struct{ CurrentPage int }{CurrentPage: currentPage})

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

func getPage() int {
	file, err := os.ReadFile(PageState)
	if err == nil {
		page, err := strconv.Atoi(string(file))
		if err == nil {
			return page
		}
	}
	return -1

}

func setCurrentPage(page int) {

	fmt.Println("setting page..")
	f, err := os.OpenFile(PageState, os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		_, err = f.WriteString(fmt.Sprintf("%d", page))

	}
	if err != nil {
		log.Println("failed to write to page", err)
	} else {
		fmt.Println("page set successfully!")
	}
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
					downloadUrl := fmt.Sprintf("https://lh3.googleusercontent.com/d/%s", page.Id)
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

	fetchFromGoogleDrive := os.Getenv("FETCH_FROM_GOOGLE_DRIVE")

	if fetchFromGoogleDrive != "" {
		comicsPages = fetchComicsPagesFromGoogleDrive()
	} else {
		comicsPages = fetchComicsPagesFromLocal()
	}

	getPage := func(w http.ResponseWriter, r *http.Request) {
		rawPage := r.PathValue("page")
		page, err := strconv.Atoi(rawPage)

		if err != nil {
			http.Redirect(w, r, "/page/1", http.StatusSeeOther)
			return
		}

		go setCurrentPage(page)

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

		pageComic := comicsPages[page-1]
		nextPage := page + 1
		prevPage := page - 1

		args := struct {
			Page     string
			PrevPage int
			NextPage int
		}{
			Page:     pageComic,
			NextPage: nextPage,
			PrevPage: prevPage,
		}
		t := template.Must(template.New("page.html").ParseFiles("./templates/page.html"))
		w.WriteHeader(200)
		t.Execute(w, args)

	}

	http.HandleFunc("GET /page/{page}", getPage)
	http.Handle("/assets/",
		http.StripPrefix("/assets", http.FileServer(http.Dir("./assets"))))
	http.HandleFunc("/", indexPage)

	fmt.Print("starting server...")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}
