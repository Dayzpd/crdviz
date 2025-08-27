package main

import (
	"context"
	"flag"
	"path/filepath"

	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type CrdFilter struct {
	ApiGroup string `json:"apiGroup"`
}

var (
	inCluster           bool
	hostname            string
	port                int
	kubeconfig          *string
	restConfig          *rest.Config
	apiextensionsClient *clientset.Clientset
	crdPropertiesTmpl   *template.Template
	crdSelectBoxTmpl    *template.Template
	indexTmpl           *template.Template
	server              *http.ServeMux
)

func initInClusterClient() {

	var err error

	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	apiextensionsClient, err = clientset.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

}

func initExternalClusterClient() {

	var err error

	restConfig, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	apiextensionsClient, err = clientset.NewForConfig(restConfig)
	if err != nil {
		panic(err.Error())
	}

}

func initTemplates() {

	var err error

	indexTmpl, err = template.ParseFiles("templates/index.html")

	if err != nil {
		log.Fatal(err)
	}

	crdSelectBoxTmpl, err = template.ParseFiles("templates/crd-select-box.html")

	if err != nil {
		log.Fatal(err)
	}
	funcMap := template.FuncMap{
		"dict":       dict,
		"replaceAll": strings.ReplaceAll,
		"join":       strings.Join,
		"derefBool":  derefBool,
	}

	crdPropertiesTmpl, err = template.New("crd-properties.html").Funcs(funcMap).ParseFiles("templates/crd-properties.html")

	if err != nil {
		log.Fatal(err)
	}

}

func init() {

	flag.BoolVar(&inCluster, "in-cluster", false,
		"Set this flag when crdviz is running inside a cluster. If not specified, you can specify the --kubeconfig to connect to a cluster.")

	flag.StringVar(&hostname, "hostname", "0.0.0.0",
		"Hostname or IP for the http server. Defaults to '0.0.0.0'.")

	flag.IntVar(&port, "port", 8080,
		"Port for the http server to listen on. Defaults to '8080'.")

	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	if inCluster {
		initInClusterClient()
	} else {
		initExternalClusterClient()
	}

	initTemplates()

	server = http.NewServeMux()

}

func main() {
	startServer()
}

func startServer() {
	server.HandleFunc("/", handleIndex)
	server.HandleFunc("/filter", handleFilterCrds)
	server.HandleFunc("/show", handleShowCrd)

	serverAddress := fmt.Sprintf("%s:%d", hostname, port)

	log.Printf("Starting webserver (%s)", serverAddress)

	http.ListenAndServe(serverAddress, server)
}

func handleIndex(
	w http.ResponseWriter,
	r *http.Request,
) {

	log.Print("Request to Index Route /")

	data := struct {
		ApiGroups []string
	}{
		ApiGroups: getCrdApiGroups(),
	}

	if err := indexTmpl.Execute(w, data); err != nil {
		http.Error(
			w,
			"Internal Server Error",
			http.StatusInternalServerError,
		)
		log.Printf("Failed to render index template: %v", err)
	}

}

func handleFilterCrds(
	w http.ResponseWriter,
	r *http.Request,
) {

	query := r.URL.Query()

	filter := CrdFilter{
		ApiGroup: query.Get("apiGroup"),
	}

	log.Printf("Request to Filter CRDs Route /filter (ApiGroup: %s)", filter.ApiGroup)

	if filter.ApiGroup == "" {
		http.Error(w, "No API group selected", http.StatusBadRequest)
		return
	}

	data := struct {
		Crds []apiextensionsv1.CustomResourceDefinition
	}{
		Crds: getCrds(filter),
	}

	if err := crdSelectBoxTmpl.Execute(w, data); err != nil {
		http.Error(
			w,
			"Internal Server Error",
			http.StatusInternalServerError,
		)
		log.Printf("Failed to render index template: %v", err)
	}

}

func handleShowCrd(
	w http.ResponseWriter,
	r *http.Request,
) {

	query := r.URL.Query()

	var crdName string = query.Get("crdName")

	log.Printf("Request to Show CRD Route /show (CRD: %s)", crdName)

	if crdName == "" {
		http.Error(w, "No CRD selected", http.StatusBadRequest)
		return
	}

	crd := getCrdByName(crdName)

	var schema *apiextensionsv1.JSONSchemaProps

	for _, version := range crd.Spec.Versions {

		if version.Storage {
			schema = version.Schema.OpenAPIV3Schema
			break
		}

	}

	data := struct {
		CRD    *apiextensionsv1.CustomResourceDefinition
		Schema *apiextensionsv1.JSONSchemaProps
	}{
		CRD:    crd,
		Schema: schema,
	}

	if err := crdPropertiesTmpl.Execute(w, data); err != nil {
		http.Error(
			w,
			"Internal Server Error",
			http.StatusInternalServerError,
		)
		log.Printf("Failed to render index template: %v", err)
	}

}

func getCrdApiGroups() []string {

	crdsList, err := apiextensionsClient.ApiextensionsV1().CustomResourceDefinitions().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}

	crdApiGroups := make(map[string]bool)

	for _, crd := range crdsList.Items {

		if existingGroup := crdApiGroups; existingGroup != nil {
			crdApiGroups[crd.Spec.Group] = true
		}

	}

	crdApiGroupNames := make([]string, 0, len(crdApiGroups))

	for groupName := range crdApiGroups {
		crdApiGroupNames = append(crdApiGroupNames, groupName)
	}

	return crdApiGroupNames
}

func getCrds(filter CrdFilter) []apiextensionsv1.CustomResourceDefinition {

	crdsList, err := apiextensionsClient.ApiextensionsV1().CustomResourceDefinitions().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}

	filteredCrds := []apiextensionsv1.CustomResourceDefinition{}

	for _, crd := range crdsList.Items {

		if crd.Spec.Group != filter.ApiGroup {
			continue
		}

		filteredCrds = append(filteredCrds, crd)
	}

	return filteredCrds
}

func getCrdByName(crdName string) *apiextensionsv1.CustomResourceDefinition {

	crd, err := apiextensionsClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.TODO(), crdName, metav1.GetOptions{})
	if err != nil {
		panic(err.Error())
	}

	return crd

}

func dict(values ...interface{}) map[string]interface{} {
	if len(values)%2 != 0 {
		panic("invalid dict call")
	}
	d := make(map[string]interface{}, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key := values[i].(string)
		d[key] = values[i+1]
	}
	return d
}

func derefBool(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}
