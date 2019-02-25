package main

import (
	"flag"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	"github.com/gorilla/mux"
)

type LogEntry struct {
	Project   string `json:"project"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	Timestamp int64  `json:"timestamp"`
	MimeType  string `json:"mime-type"`
}

var (
	mongoUrl string
	bind     string
)

func UploadHandler(session *mgo.Session) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		project := vars["project"]
		subject := vars["subject"]

		if project == "" || subject == "" {
			w.Write([]byte("empty project or subject"))
			return
		}

		r.ParseForm()
		body := r.PostFormValue("body")
		mimeType := r.PostFormValue("mimeType")
		if mimeType == "" {
			mimeType = "html/text"
		}

		timestamp := time.Now().Unix()
		log.Printf("upload %v %v %v %v\n", project, subject, body, mimeType)

		c := session.DB("").C("log")
		if err := c.Insert(&LogEntry{
			Project:   project,
			Subject:   subject,
			Body:      body,
			Timestamp: timestamp,
			MimeType:  mimeType,
		}); err != nil {
			w.Write([]byte(err.Error()))
		}
		w.Write([]byte("success"))
	}
}

type Index struct {
	Project []string
}

func IndexHandler(session *mgo.Session) func(w http.ResponseWriter, r *http.Request) {
	tpl := `
		<html>
			<head>
			</head>
			<body>
				log available:
				<br>
				<table>
					{{range $i, $v := .Project}}
						<tr>
						<td>&nbsp;</td><td><a href="/log/{{$v}}/">{{$v}}</a></td>
						</tr>
					{{end}}
				</table>
			</body>
		</html>
		`

	t, err := template.New("index").Parse(tpl)
	if err != nil {
		panic(err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var result []string
		c := session.DB("").C("log")
		err := c.Find(nil).Distinct("project", &result)
		if err != nil {
			w.Write([]byte(err.Error()))
			return
		}

		w.Header().Set("Content-Type", "text/html")
		err = t.Execute(w, &Index{Project: result})
		if err != nil {
			w.Write([]byte(err.Error()))
			return
		}
	}
}

type Project struct {
	Project string
	Subject []string
}

func ProjecttHandler(session *mgo.Session) func(w http.ResponseWriter, r *http.Request) {
	tpl := `
		<html>
			<head>
			</head>
			<body>
				log available:
				<br>
				<table>
					{{range $i, $v := .Subject}}
						<tr>
						<td>&nbsp;</td><td><a href="/log/{{$.Project}}/{{$v}}/">{{$v}}</a></td>
						</tr>
					{{end}}
				</table>
			</body>
		</html>
		`
	t, err := template.New("project").Parse(tpl)
	if err != nil {
		panic(err)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		project := vars["project"]

		c := session.DB("").C("log")
		var result = []string{}
		err := c.Find(bson.M{"project": project}).Distinct("subject", &result)
		if err != nil {
			w.Write([]byte(err.Error()))
			return
		}

		w.Header().Set("Content-Type", "text/html")

		err = t.Execute(w, &Project{Project: project, Subject: result})
		if err != nil {
			w.Write([]byte(err.Error()))
			return
		}
	}
}

type LogHeader struct {
	ID        bson.ObjectId `bson:"_id"`
	Timestamp int64
}

func (lh *LogHeader) Date() string {
	return time.Unix(lh.Timestamp, 0).String()
}

type Subject struct {
	Project string
	Subject string
	Header  []LogHeader
}

func SubjectHandler(session *mgo.Session) func(w http.ResponseWriter, r *http.Request) {
	tpl := `
		<html>
			<head>
			</head>
			<body>
				project: {{.Project}}
				<br>
				<br>
				log available:
				<br>
				<table>
					{{range $i, $v := .Header}}
						<tr>
						<td>&nbsp;</td><td><a href="/log/{{$.Project}}/{{$.Subject}}/{{$v.ID.Hex}}">{{$v.ID.Hex}}</a></td>
						<td>{{$v.Date}}</td>
						</tr>
					{{end}}
				</table>
			</body>
		</html>
		`
	t, err := template.New("subject").Parse(tpl)
	if err != nil {
		panic(err)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		project := vars["project"]
		subject := vars["subject"]

		c := session.DB("").C("log")
		var result = []LogHeader{}
		err := c.Find(bson.M{"project": project, "subject": subject}).Select(bson.M{
			"_id":       1,
			"timestamp": 1,
		}).All(&result)
		if err != nil {
			w.Write([]byte(err.Error()))
			return
		}

		w.Header().Set("Content-Type", "text/html")
		err = t.Execute(w, &Subject{Project: project, Subject: subject, Header: result})
		if err != nil {
			w.Write([]byte(err.Error()))
			return
		}
	}
}

func LogHandler(session *mgo.Session) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id := vars["id"]

		c := session.DB("").C("log")
		logEntry := &LogEntry{}
		err := c.Find(bson.M{"_id": bson.ObjectIdHex(id)}).One(logEntry)
		if err != nil {
			w.Write([]byte(err.Error()))
			return
		}

		w.Header().Set("Content-Type", logEntry.MimeType)
		switch logEntry.MimeType {
		case "application/json":
			//if err := json.NewEncoder(w).Encode(strconv.Unquote(logEntry.Body)); err != nil {
			w.Write([]byte(logEntry.Body))
		default:
			w.Write([]byte(logEntry.Body))
		}
	}
}

func main() {
	flag.StringVar(&mongoUrl, "mongo-url", "mongodb://127.0.0.1:27017/log", "mongodb url")
	flag.StringVar(&bind, "bind", ":80", "bind addr")
	flag.Parse()
	log.Println("start:")
	log.Println("mongodb:", mongoUrl)
	log.Println("bind:", bind)

	session, err := mgo.Dial(mongoUrl)
	if err != nil {
		panic(err)
	}
	defer session.Close()

	r := mux.NewRouter()

	r.HandleFunc("/", IndexHandler(session))

	r.HandleFunc("/upload/{project}/{subject}", UploadHandler(session))

	r.HandleFunc("/log/{project}/", ProjecttHandler(session))

	r.HandleFunc("/log/{project}/{subject}/", SubjectHandler(session))

	r.HandleFunc("/log/{project}/{subject}/{id}", LogHandler(session))

	log.Fatal(http.ListenAndServe(bind, r))
}
