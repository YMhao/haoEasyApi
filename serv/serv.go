package serv

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/YMhao/EasyApi/common"
	"github.com/YMhao/EasyApi/generate/swagger"
	"github.com/YMhao/EasyApi/web"
	"github.com/ghodss/yaml"
	"github.com/gin-gonic/gin"
)

func getApiMap(setsOfAPIs APISets) map[string]API {
	apiMap := make(map[string]API)
	for _, APISet := range setsOfAPIs {
		for _, API := range APISet {
			apiDoc := API.Doc()
			apiMap[apiDoc.ID] = API
		}
	}
	return apiMap
}

// RunAPIServ 启动服务
func RunAPIServ(conf *APIServConf, setsOfAPIs APISets) {
	conf.format()
	router := gin.Default()
	web.SetHTMLTemplate(router)

	apiMap := getApiMap(setsOfAPIs)
	Key := "id"
	path := getBasePath(conf) + "/:" + Key
	router.POST(path, func(c *gin.Context) {
		runAPICall(apiMap, c, Key)
	})
	router.OPTIONS(path, func(c *gin.Context) {
		runAPIOpthionCall(apiMap, c, Key)
	})

	if conf.DebugOn {
		initHTML(conf, setsOfAPIs, router)
	}
	router.Run(conf.ListenAddr)
}

func getSwaggerProtocolURL(conf *APIServConf, path string) string {
	url := swagger.GetHostFromConf(conf.HTTPProxy, conf.ListenAddr) + path
	if strings.HasPrefix(conf.HTTPProxy, "https://") {
		return url
	}
	if strings.HasPrefix(conf.HTTPProxy, "http://") {
		return url
	}
	return "http://" + url
}

func jsonToYaml(jsonStr string) (string, error) {
	yamlBytes, err := yaml.JSONToYAML([]byte(jsonStr))
	return string(yamlBytes), err
}

func initHTML(conf *APIServConf, setsOfAPIs APISets, router *gin.Engine) {
	swaggerJSONStr, swaggerYAMLStr, err := genSwagger(conf, setsOfAPIs)
	if err != nil {
		fmt.Println("Warn: ", err)
	}
	rawSwaggerProtocolJSONPath := "/swaggerJSON"
	rawSwaggerProtocolYAMLPath := "/swaggerYAML"
	index := web.IndexInfo{
		Name:        conf.ServiceName,
		Description: conf.Description,
		URL:         getSwaggerProtocolURL(conf, rawSwaggerProtocolYAMLPath),
		SwaggerJSON: swaggerJSONStr,
		SwaggerYAML: swaggerYAMLStr,
		HTTPS:       strings.HasPrefix(conf.HTTPProxy, "https://"),
	}
	router.GET("/", func(c *gin.Context) {
		cors(c, "*")
		c.HTML(200, "Index", index.HTMLIndexInfo())
	})
	router.OPTIONS("/", func(c *gin.Context) {
		runOpthionCall(c)
	})
	router.GET(rawSwaggerProtocolJSONPath, func(c *gin.Context) {
		cors(c, "*")
		c.String(200, swaggerJSONStr)
	})
	router.OPTIONS(rawSwaggerProtocolJSONPath, func(c *gin.Context) {
		runOpthionCall(c)
	})
	router.GET(rawSwaggerProtocolYAMLPath, func(c *gin.Context) {
		cors(c, "*")
		c.String(200, swaggerYAMLStr)
	})
	router.OPTIONS(rawSwaggerProtocolYAMLPath, func(c *gin.Context) {
		runOpthionCall(c)
	})
}

func genSwagger(conf *APIServConf, setsOfAPIs APISets) (swagerJSON string, swagerYaml string, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("internal error: %v", p)
			return
		}
	}()

	docList := genAPIDocList(conf, setsOfAPIs)
	swagger := swagger.GenCode(&common.APIServConf{
		Version:     conf.Version,
		BuildTime:   conf.BuildTime,
		ServiceName: conf.ServiceName,
		Description: conf.Description,
		ListenAddr:  conf.ListenAddr,
		DebugOn:     conf.DebugOn,
		HTTPProxy:   conf.HTTPProxy,
	}, docList)
	jsonData, err := swagger.MarshalJSON()
	if err != nil {
		return "", "", nil
	}
	yamlData, err := swagger.MarshalYAML()
	if err != nil {
		return "", "", nil
	}
	return string(jsonData), string(yamlData), nil
}

func getPath(conf *APIServConf, apiID string) string {
	return "/" + conf.ServiceName + "/" + conf.Version + "/" + apiID
}

func getBasePath(conf *APIServConf) string {
	return "/" + conf.ServiceName + "/" + conf.Version
}

func runAPICall(apiMap map[string]API, c *gin.Context, key string) {
	cors(c, "*")
	contentType := c.Request.Header.Get("Content-Type")
	contentType = strings.ToLower(contentType)
	if strings.Contains(contentType, "application/json") {
		body, err := ioutil.ReadAll(c.Request.Body)
		if err != nil {
			handleError(c, &APIError{
				Code:    common.ERROR_TYPE_DEFAULT,
				Message: err.Error(),
			})
			return
		}
		api, ok := apiMap[c.Param(key)]
		if !ok {
			handleNotFound(c)
			return
		}
		response, apiErr := api.Call(body, c)
		handleCall(c, response, apiErr)
	} else {
		handleError(c, &APIError{
			Code:    common.ERROR_TYPE_DEFAULT,
			Message: "invalid content-type " + contentType,
		})
	}
}

func runAPIOpthionCall(apiMap map[string]API, c *gin.Context, key string) {
	_, ok := apiMap[c.Param(key)]
	if !ok {
		handleNotFound(c)
		return
	}
	runOpthionCall(c)
}

func handleNotFound(c *gin.Context) {
	c.String(500, "")
}

func handleCall(c *gin.Context, response interface{}, apiErr *APIError) {
	c.Writer.Header().Set("content-type", "application/json")
	c.JSON(200, response)
}

func handleError(c *gin.Context, apiErr *APIError) {
	c.Writer.Header().Set("content-type", "application/json")
	c.JSON(403, apiErr)
}

func runOpthionCall(c *gin.Context) {
	c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	c.Writer.Header().Set("Access-Control-Allow-Headers", "Access-Control-Allow-Origin,Access-Control-Allow-Method,Content-Type")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	c.Writer.Header().Add("Access-Control-Allow-Headers", "Content-Type")
	c.Writer.Header().Set("content-type", "application/json")

	c.JSON(200, gin.H{})
}

func cors(c *gin.Context, origin string) {
	c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	c.Writer.Header().Set("Access-Control-Allow-Headers", "Access-Control-Allow-Origin,Access-Control-Allow-Method,Content-Type")
	c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
}
