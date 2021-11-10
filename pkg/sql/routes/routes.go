package routes

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/grafana/grafana-aws-sdk/pkg/sql/api"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/sqlds/v2"
)

type ResourceHandler struct {
	API api.Resources
}

func Write(rw http.ResponseWriter, b []byte) {
	_, err := rw.Write(b)
	if err != nil {
		log.DefaultLogger.Error(err.Error())
	}
}

func ParseBody(body io.ReadCloser) (sqlds.Options, error) {
	reqBody := sqlds.Options{}
	b, err := ioutil.ReadAll(body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(b, &reqBody)
	if err != nil {
		return nil, err
	}
	return reqBody, nil
}

func SendResources(rw http.ResponseWriter, res interface{}, err error) {
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		Write(rw, []byte(err.Error()))
		return
	}
	bytes, err := json.Marshal(res)
	if err != nil {
		log.DefaultLogger.Error(err.Error())
		rw.WriteHeader(http.StatusInternalServerError)
		Write(rw, []byte(err.Error()))
		return
	}
	rw.Header().Add("Content-Type", "application/json")
	Write(rw, bytes)
}

func (r *ResourceHandler) regions(rw http.ResponseWriter, req *http.Request) {
	regions, err := r.API.Regions(req.Context())
	SendResources(rw, regions, err)
}

func (r *ResourceHandler) databases(rw http.ResponseWriter, req *http.Request) {
	reqBody, err := ParseBody(req.Body)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		Write(rw, []byte(err.Error()))
		return
	}
	res, err := r.API.Databases(req.Context(), reqBody)
	SendResources(rw, res, err)
}

func (r *ResourceHandler) DefaultRoutes() map[string]func(http.ResponseWriter, *http.Request) {
	return map[string]func(http.ResponseWriter, *http.Request){
		"/regions":   r.regions,
		"/databases": r.databases,
	}
}
