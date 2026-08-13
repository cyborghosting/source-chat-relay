package ui

import (
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"sort"

	"github.com/rumblefrog/source-chat-relay/server/config"
	"github.com/rumblefrog/source-chat-relay/server/relay"

	"github.com/rumblefrog/source-chat-relay/server/entity"

	"github.com/sirupsen/logrus"
)

//go:embed template/index.html
var indexHtmlStr string
//go:embed template/styles.css
var stylesCssStr string

type renderData struct {
	Relay    *relay.Relay
	Entities []*renderEntity
}

type renderEntity struct {
	Entity      *entity.Entity
	Highlighted bool
}

func UIListen() {
	http.HandleFunc("/", viewHandler)
	http.HandleFunc("/styles.css", styleHandler)

	if config.Config.UI.Port == 0 {
		config.Config.UI.Port = 8080
	}

	logrus.Infof("UI listener started on port %d", config.Config.UI.Port)

	logrus.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", config.Config.UI.Port), nil))
}

func viewHandler(w http.ResponseWriter, r *http.Request) {
	temp, err := template.New("index").Funcs(template.FuncMap{
		"humanizeChannelString": entity.HumanizeChannelString,
		"byteToMB": func(b int) string {
			return fmt.Sprintf("%.6f", float64(b)/(1024*1024))
		},
	}).Parse(indexHtmlStr)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "text/html")

	entities := entity.Entities()

	tRenderData := renderData{
		Relay: relay.Instance,
	}

	for _, v := range entities {
		tRenderData.Entities = append(tRenderData.Entities, &renderEntity{
			Entity: v,
		})
	}

	// Upon retrieving from cache, it may not be in the same order each time
	// leading to confusions when running trace
	sort.Slice(tRenderData.Entities, func(i, j int) bool {
		return tRenderData.Entities[i].Entity.ID < tRenderData.Entities[j].Entity.ID
	})

	if r.Method == http.MethodPost {
		action := r.FormValue("btn")

		switch action {
		case "update":
			// Since we already retrieved from cache prior (above and on bot ready), this entity should point to the same address
			// as tRenderData and relay instance entities, therefore the template render should reflect the update
			tEntity, err := entity.GetEntity(r.FormValue("id"))

			if err != nil {
				break
			}

			tEntity.ReceiveChannels = entity.ParseDelimitedChannels(r.FormValue("receiveChannels"))
			tEntity.SendChannels = entity.ParseDelimitedChannels(r.FormValue("sendChannels"))
			tEntity.DisabledReceiveTypes = entity.ParseDelimitedChannels(r.FormValue("disabledReceiveTypes"))
			tEntity.DisabledSendTypes = entity.ParseDelimitedChannels(r.FormValue("disabledSendTypes"))

			tEntity.Propagate()
		case "delete":
			tEntity, err := entity.GetEntity(r.FormValue("id"))

			if err != nil {
				break
			}

			index := -1

			for i, v := range tRenderData.Entities {
				if v.Entity.ID == tEntity.ID {
					index = i
				}
			}

			if index == -1 {
				break
			}

			tRenderData.Entities[index] = tRenderData.Entities[len(tRenderData.Entities)-1]

			tRenderData.Entities = tRenderData.Entities[:len(tRenderData.Entities)-1]

			tEntity.Delete()
		case "trace":
			for _, v := range tRenderData.Entities {
				sendChannels := r.FormValue("sendChannels")
				receiveChannels := r.FormValue("receiveChannels")

				if len(sendChannels) != 0 && v.Entity.ReceiveIntersectsWith(entity.ParseDelimitedChannels(sendChannels)) {
					v.Highlighted = true
				}

				if len(receiveChannels) != 0 && v.Entity.SendIntersectsWith(entity.ParseDelimitedChannels(receiveChannels)) {
					v.Highlighted = true
				}
			}
		}
	}

	err = temp.Execute(w, tRenderData)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}
}

func styleHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")

	io.WriteString(w, stylesCssStr)
}
