package controller

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

func init() {
	go func() {
		f, err := os.Create("/rancher_log.csv")
		defer f.Close()
		if err != nil {
			fmt.Println("Wasn't able to create file")
			return
		}
		time.Sleep(time.Minute * 5)
		if err := debugMap.ToCsv(f, "agent"); err != nil {
			fmt.Println(err)
		}
	}()
}

var debugMap debug

type debug map[Ressource][]RegisterData

var registeredOn = "initialization"
var rancher = "rancher"

type Ressource struct {
	Resource        string
	ResourceGroup   string
	ResourceVersion string
	CreatedBy       string
}

func (d *debug) ToFile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return d.ToCsv(f, "")

}

func (r Ressource) String() string {
	return fmt.Sprintf("%s/%s, Resource=%s", r.ResourceGroup, r.ResourceVersion, r.Resource)
}

func (d *debug) ToCsv(writer io.Writer, createdByFilter string) error {

	w := csv.NewWriter(writer)
	w.Comma = ';'

	if err := w.Write([]string{"Resource", "ResourceGroup", "ResourVersion", "ControllerName", "File", "Line", "CreatedOn", "CreateBy"}); err != nil {
		return err
	}
	for k, controllers := range *d {
		if createdByFilter != "" && createdByFilter != k.CreatedBy {
			continue
		}

		initialValues := []string{k.Resource, k.ResourceGroup, k.ResourceVersion}
		for _, v := range controllers {
			if err := w.Write(append(initialValues, v.ControllerName, v.ControllerFile, strconv.Itoa(v.ControllerLine), v.InvokedOne, k.CreatedBy)); err != nil {
				return err
			}
			w.Flush()
		}
	}
	return nil
}

type RegisterData struct {
	ControllerName string
	ControllerFile string
	ControllerLine int
	InvokedOne     string
}

type callerKey string
type fileCallerData struct {
	line int
	file string
}

func contextWithCaller(ctx context.Context, file string, line int) context.Context {
	return context.WithValue(ctx, callerKey("caller"), fileCallerData{
		line: line,
		file: file,
	})
}

func (d *debug) Log(ctx context.Context, name, controllerGVR string) {
	if debugMap == nil {
		debugMap = map[Ressource][]RegisterData{}
	}

	caller, ok := ctx.Value(callerKey("caller")).(fileCallerData)
	if !ok {
		caller.file = "unknown"
	}
	if controllerGVR == "" {
		controllerGVR = "using_matcher"
	}

	debugMap[resourceFromString(controllerGVR)] = append(debugMap[resourceFromString(controllerGVR)], RegisterData{
		ControllerName: name,
		ControllerFile: caller.file,
		ControllerLine: caller.line,
		InvokedOne:     registeredOn,
	})
}

// resourceFromString splits the GVR into resource, version, kind
func resourceFromString(s string) Ressource {
	firstSplit := strings.Split(s, ",")

	if len(firstSplit) != 2 {
		fmt.Println("ERROR WILE SPLITING GVR, DEFAULTING TO NAME")
		return Ressource{
			Resource:        s,
			ResourceGroup:   "",
			ResourceVersion: "",
			CreatedBy:       rancher,
		}
	}
	resource := strings.Replace(firstSplit[1], " Resource=", "", 1)
	secondSplit := strings.Split(firstSplit[0], "/")
	if len(secondSplit) != 2 {
		fmt.Println("ERROR WILE SPLITING Type and version of GRV, DEFAULTING TO NAME")
		return Ressource{
			Resource:        resource,
			ResourceGroup:   firstSplit[0],
			ResourceVersion: "Default",
			CreatedBy:       rancher,
		}
	}
	return Ressource{
		Resource:        resource,
		ResourceGroup:   secondSplit[0],
		ResourceVersion: secondSplit[1],
		CreatedBy:       rancher,
	}

}
