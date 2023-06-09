package main

import (
	"DocuLegal/Models"
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jung-kurt/gofpdf"
	openai "github.com/sashabaranov/go-openai"
)

func main() {
	// Create a new file server for the directory
	dir := "/tmp"
	fileServer := http.FileServer(http.Dir(dir))
	// Register the file server on the /static endpoint
	http.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
		http.StripPrefix("/static", fileServer).ServeHTTP(w, r)
	})

	http.HandleFunc("/DocuLegal", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// Serve the index.html file for GET requests
			http.ServeFile(w, r, "index.html")
		} else if r.Method == http.MethodPost {
			// Handle POST requests for the index page here
			uploadHandler(w, r)
		} else {
			// Return a "Method Not Allowed" error for all other types of requests
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	})
	http.HandleFunc("/delete", deleteFilesHandler)

	log.Println("Server listening on port 8080...")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal(err)
	}
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Files pushed by user")
	err := r.ParseMultipartForm(10 << 20) // 10 MB file size limit
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//Handle Excel file
	file1, handler1, err := r.FormFile("file1")
	if err != nil {
		http.Error(w, "Error retrieving file1", http.StatusBadRequest)
		return
	}
	log.Println(handler1.Filename)
	defer file1.Close()
	if hasXLSXExtension(handler1.Filename) == false {
		http.Error(w, "Wrong file extension, please input a .xlsx and a .docx", http.StatusInternalServerError)
		return
	}
	excelFile, err := os.CreateTemp("/tmp", "*-"+handler1.Filename)
	if err != nil {
		log.Fatal(err)
	}
	log.Println(excelFile.Name())
	defer os.Remove(excelFile.Name()) // clean up


// Read the file bytes into a buffer
buf := make([]byte, handler1.Size)
_, err = file1.Read(buf)
if err != nil {
    log.Fatal(err)// Handle the error
}
	if _, err := excelFile.Write(buf); err != nil {
		log.Fatal(err)
	}
	if err := excelFile.Close(); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Excel file temp uploaded: %s\n", excelFile.Name())

//Handle Word file
	file2, handler2, err := r.FormFile("file2")
	if err != nil {
		http.Error(w, "Error retrieving file2", http.StatusBadRequest)
		return
	}
	log.Println(handler2.Filename)
	defer file2.Close()
	if hasDOCXExtension(handler2.Filename) == false {
		http.Error(w, "Wrong file extension, please input a .xlsx and a .docx", http.StatusInternalServerError)
		return
	}
	wordFile, err := os.CreateTemp("/tmp", "*-"+handler2.Filename)
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(wordFile.Name()) // clean up

	// Read the file bytes into a buffer
buf = make([]byte, handler2.Size)
_, err = file2.Read(buf)
if err != nil {
    // Handle the error
}
	if _, err := wordFile.Write(buf); err != nil {
		log.Fatal(err)
	}
	if err := wordFile.Close(); err != nil {
		log.Fatal(err)
	
	fmt.Printf("Word file uploaded: %s\n", wordFile.Name())

	//Handle user prompt
	testPrompt := r.FormValue("text")

	// Process files
	id := uuid.New()
	chatGPTPrompt, err := ProcessFiles(excelFile.Name(), wordFile.Name(), testPrompt, id.String())
	if err != nil {
		fmt.Fprint(w, err)
	}

	//Output a page with results
	html := `<!DOCTYPE html>
		<html>
			<head>
  				<meta charset="UTF-8">
  				<title>DocuLegal</title>
			</head>
			<body>
				<h3>Voici le récapitulatif de votre prompt :</h3>
				<p>%s</p>
				<br>
				<p>Voici le lien pour accéder à votre template variabilisé : <a href="http://localhost:8080.org/static/%s-template.pdf" target="_blank">Mon document</a>
				<br>
				<br>
				<p> Ci-dessous voici vos fichiers :</p>
			</body>
		</html>
	`
	html = fmt.Sprintf(html, chatGPTPrompt, id.String())
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, html)
}

func ProcessFiles(excelFilePath string, wordFilePath string, chatGPTPrompt string, uniqueID string) (string, error) {
	log.Println("Launched processing..")
	// docx to text
	initialTemplateContent := models.ExtractTextFromWordDocument(wordFilePath)
	//fmt.Println(initialTemplateContent)
	/*err := ioutil.WriteFile("output.txt", []byte(initialTemplateContent), 0644)
	if err != nil {
		panic(err)
	}*/
	// xlsx header to text + first line
	headerList, firstLine := models.ExtractExcelHeader(excelFilePath)
	//fmt.Println(headerList, firstLine)

	//Add header to chatGPTPrompt
	chatGPTPrompt = strings.Replace(chatGPTPrompt, "__header__", strings.Join(headerList, ";"), -1)
	//Add first excel line to chatGPTPrompt
	chatGPTPrompt = strings.Replace(chatGPTPrompt, "__excelFirstRow__", strings.Join(firstLine, ";"), -1)

	//chatgpt prompt assembly
	template := ChatGPT(chatGPTPrompt, initialTemplateContent)
	
	// Convert the text to a []rune
	runeSlice := []rune(template)

	// Create a new bytes.Buffer
	buffer := bytes.Buffer{}

	// Iterate over the []rune and convert to Windows-1252
	for _, r := range runeSlice {
		if utf8.ValidRune(r) {
			buffer.WriteRune(r)
		} else {
			buffer.WriteRune(utf8.RuneError)
		}
	}

	// Convert the bytes.Buffer to []byte
	byteSlice := buffer.Bytes()

	// Convert the []byte to a string in Windows-1252 encoding
	str := string(windows1252EncodedBytes(byteSlice))
	// Create a new PDF
	pdf := gofpdf.New("P", "mm", "A4", "")

	// Add a new page to the PDF
	pdf.AddPage()

	pdf.SetFont("Times", "", 12)

	// Write the text to the PDF
	pdf.MultiCell(0, 10, str, "", "", false)

	// Output to a file
	err := pdf.OutputFileAndClose("/tmp/"+uniqueID+"-template.pdf")
	if err != nil {
		log.Println(err)
		return chatGPTPrompt, err
	}
	return chatGPTPrompt, nil
}

func windows1252EncodedBytes(b []byte) []byte {
	var windows1252Encoded []byte

	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		b = b[size:]
		if r == utf8.RuneError {
			windows1252Encoded = append(windows1252Encoded, byte(r))
		} else {
			windows1252Encoded = append(windows1252Encoded, byte(r))
		}
	}
	return windows1252Encoded
}

func ChatGPT(prompt string, paragraph string) string {
	client := openai.NewClient("CHAT-GPT_KEY")
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt + paragraph,
				},
			},
		},
	)

	if err != nil {
		fmt.Printf("ChatCompletion error: %v\n", err)
		return "No answer from chatGPT"
	}

	return resp.Choices[0].Message.Content
}

func pdfGeneration(template string, clientsDataFilePath string) {
	// Ouverture du fichier CSV
	f, err := os.Open(clientsDataFilePath)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()

	// Parsage du fichier CSV
	reader := csv.NewReader(f)
	rows, err := reader.ReadAll()
	if err != nil {
		fmt.Println(err)
		return
	}

	// Itérer sur chaque ligne du fichier CSV
	for i, row := range rows {
		if i == 0 { // Ignorer la première ligne
			continue
		}
		siren, _ := strconv.Atoi(row[0])
		montant, _ := strconv.ParseFloat(row[1], 64)
		filename := fmt.Sprintf("%d.pdf", siren) // Nom du fichier PDF à générer

		// Créer un nouveau PDF et y ajouter le texte
		pdf := gofpdf.New(gofpdf.OrientationPortrait, gofpdf.UnitPoint, gofpdf.PageSizeLetter, "")
		pdf.AddPage()
		pdf.SetFont("Arial", "B", 16)
		text := fmt.Sprintf("Bonjour %d, par rapport à votre deal de %v.", siren, montant)
		pdf.Cell(0, 60, text)
		pdf.OutputFileAndClose(filename)
	}

	fmt.Println("PDF généré avec succès !")
}

func deleteFilesHandler(w http.ResponseWriter, r *http.Request) {
	err := os.RemoveAll("/tmp")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(w, "Files deleted successfully")
}

func deleteFiles() {
	err := os.RemoveAll("/tmp")
	if err != nil {
		log.Fatal(err)
	}
}
func deleteFilesInternal() error {
	req, err := http.NewRequest(http.MethodGet, "http://localhost:8080/delete", nil)
	if err != nil {
		return err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to delete files: %s", res.Status)
	}

	return nil
}
func hasDOCXExtension(filename string) bool {
	extension := filepath.Ext(filename)
	log.Println(extension)
	return extension == ".docx"
}

func hasXLSXExtension(filename string) bool {
	extension := filepath.Ext(filename)
	log.Println(extension)
	return extension == ".xlsx"
}
