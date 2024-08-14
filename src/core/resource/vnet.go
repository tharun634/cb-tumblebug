/*
Copyright 2019 The Cloud-Barista Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package resource is to manage multi-cloud infra resource
package resource

import (
	"encoding/json"
	"fmt"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/label"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	validator "github.com/go-playground/validator/v10"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
)

// 2020-04-09 https://github.com/cloud-barista/cb-spider/blob/master/cloud-control-manager/cloud-driver/interfaces/resources/VPCHandler.go

// SpiderVPCReqInfoWrapper is a wrapper struct to create JSON body of 'Create VPC request'
type SpiderVPCReqInfoWrapper struct {
	ConnectionName string
	ReqInfo        SpiderVPCReqInfo
}

// SpiderVPCReqInfo is a struct to create JSON body of 'Create VPC request'
type SpiderVPCReqInfo struct {
	Name           string
	IPv4_CIDR      string
	SubnetInfoList []SpiderSubnetReqInfo
	CSPId          string
}

// SpiderSubnetReqInfoWrapper is a wrapper struct to create JSON body of 'Create subnet request'
type SpiderSubnetReqInfoWrapper struct {
	ConnectionName string
	ReqInfo        SpiderSubnetReqInfo
}

// SpiderSubnetReqInfo is a struct to create JSON body of 'Create subnet request'
type SpiderSubnetReqInfo struct {
	Name         string `validate:"required"`
	IPv4_CIDR    string `validate:"required"`
	KeyValueList []common.KeyValue
}

// SpiderVPCInfo is a struct to handle VPC information from the CB-Spider's REST API response
type SpiderVPCInfo struct {
	IId            common.IID // {NameId, SystemId}
	IPv4_CIDR      string
	SubnetInfoList []SpiderSubnetInfo

	TagList      []common.KeyValue
	KeyValueList []common.KeyValue
}

// SpiderSubnetInfo is a struct to handle subnet information from the CB-Spider's REST API response
type SpiderSubnetInfo struct {
	IId       common.IID // {NameId, SystemId}
	Zone      string     // Target Zone Name
	IPv4_CIDR string

	TagList      []common.KeyValue
	KeyValueList []common.KeyValue
}

// TbVNetReq is a struct to handle 'Create vNet' request toward CB-Tumblebug.
type TbVNetReq struct { // Tumblebug
	Name           string        `json:"name" validate:"required"`
	ConnectionName string        `json:"connectionName" validate:"required"`
	CidrBlock      string        `json:"cidrBlock"`
	SubnetInfoList []TbSubnetReq `json:"subnetInfoList"`
	Description    string        `json:"description"`
	CspVNetId      string        `json:"cspVNetId"`
}

// TbVNetReqStructLevelValidation is a function to validate 'TbVNetReq' object.
func TbVNetReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(model.TbVNetReq)

	err := common.CheckString(u.Name)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.Name, "name", "Name", err.Error(), "")
	}
}

// TbVNetInfo is a struct that represents TB vNet object.
type TbVNetInfo struct { // Tumblebug
	Id                   string            `json:"id"`
	Name                 string            `json:"name"`
	ConnectionName       string            `json:"connectionName"`
	CidrBlock            string            `json:"cidrBlock"`
	SubnetInfoList       []TbSubnetInfo    `json:"subnetInfoList"`
	Description          string            `json:"description"`
	CspVNetId            string            `json:"cspVNetId"`
	CspVNetName          string            `json:"cspVNetName"`
	Status               string            `json:"status"`
	KeyValueList         []common.KeyValue `json:"keyValueList"`
	AssociatedObjectList []string          `json:"associatedObjectList"`
	IsAutoGenerated      bool              `json:"isAutoGenerated"`

	// SystemLabel is for describing the Resource in a keyword (any string can be used) for special System purpose
	SystemLabel string `json:"systemLabel" example:"Managed by CB-Tumblebug" default:""`

	// Disabled for now
	//Region         string `json:"region"`
	//ResourceGroupName string `json:"resourceGroupName"`
}

// TbSubnetReq is a struct that represents TB subnet object.
type TbSubnetReq struct { // Tumblebug
	Name         string `validate:"required"`
	IdFromCsp    string
	IPv4_CIDR    string `validate:"required"`
	Zone         string `json:"zone,omitempty"`
	KeyValueList []common.KeyValue
	Description  string
}

// TbSubnetReqStructLevelValidation is a function to validate 'TbSubnetReq' object.
func TbSubnetReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(model.TbSubnetReq)

	err := common.CheckString(u.Name)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.Name, "name", "Name", err.Error(), "")
	}
}

// CreateVNet accepts vNet creation request, creates and returns an TB vNet object
func CreateVNet(nsId string, u *model.TbVNetReq, option string) (model.TbVNetInfo, error) {
	log.Info().Msg("CreateVNet")
	temp := model.TbVNetInfo{}
	resourceType := model.StrVNet

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return temp, err
	}

	err = validate.Struct(u)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			return temp, err
		}

		temp := model.TbVNetInfo{}
		return temp, err
	}

	check, err := CheckResource(nsId, resourceType, u.Name)

	if check {
		err := fmt.Errorf("The vNet " + u.Name + " already exists.")
		return temp, err
	}

	if err != nil {
		err := fmt.Errorf("Failed to check the existence of the vNet " + u.Name + ".")
		return temp, err
	}

	uuid := common.GenUid()

	requestBody := model.SpiderVPCReqInfoWrapper{}
	requestBody.ConnectionName = u.ConnectionName
	requestBody.ReqInfo.Name = uuid
	requestBody.ReqInfo.IPv4_CIDR = u.CidrBlock
	requestBody.ReqInfo.CSPId = u.CspVNetId

	// requestBody.ReqInfo.SubnetInfoList = u.SubnetInfoList
	for _, v := range u.SubnetInfoList {
		jsonBody, err := json.Marshal(v)
		if err != nil {
			log.Error().Err(err).Msg("")
		}

		spiderSubnetInfo := model.SpiderSubnetReqInfo{}
		err = json.Unmarshal(jsonBody, &spiderSubnetInfo)
		if err != nil {
			log.Error().Err(err).Msg("")
		}
		//spiderSubnetInfo.Name = common.GenUid()
		//TODO: need to use GenUid() after enable CB-TB Subnet opject and its ID (for now, pass the given subnet name)
		spiderSubnetInfo.Name = v.Name

		requestBody.ReqInfo.SubnetInfoList = append(requestBody.ReqInfo.SubnetInfoList, spiderSubnetInfo)
	}

	client := resty.New()
	method := "POST"
	var callResult model.SpiderVPCInfo
	var url string

	if option == "register" && u.CspVNetId == "" {
		url = fmt.Sprintf("%s/vpc/%s", model.SpiderRestUrl, u.Name)
		method = "GET"
	} else if option == "register" && u.CspVNetId != "" {
		url = fmt.Sprintf("%s/regvpc", model.SpiderRestUrl)
	} else { // option != "register"
		url = fmt.Sprintf("%s/vpc", model.SpiderRestUrl)
	}

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		&callResult,
		common.MediumDuration,
	)
	if err != nil {
		log.Error().Err(err).Msg("")
		return temp, err
	}

	content := model.TbVNetInfo{}
	//content.Id = common.GenUid()
	content.Id = u.Name
	content.Name = u.Name
	content.Uuid = uuid
	content.ConnectionName = u.ConnectionName
	content.CspVNetId = callResult.IId.SystemId
	content.CspVNetName = callResult.IId.NameId
	content.CidrBlock = callResult.IPv4_CIDR
	content.Description = u.Description
	content.KeyValueList = callResult.KeyValueList
	content.AssociatedObjectList = []string{}

	if option == "register" && u.CspVNetId == "" {
		content.SystemLabel = "Registered from CB-Spider resource"
	} else if option == "register" && u.CspVNetId != "" {
		content.SystemLabel = "Registered from CSP resource"
	}

	Key := common.GenResourceKey(nsId, model.StrVNet, content.Id)
	Val, _ := json.Marshal(content)

	err = kvstore.Put(Key, string(Val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return content, err
	}

	for _, v := range callResult.SubnetInfoList {
		jsonBody, err := json.Marshal(v)
		if err != nil {
			log.Error().Err(err).Msg("")
		}

		tbSubnetReq := model.TbSubnetReq{}
		err = json.Unmarshal(jsonBody, &tbSubnetReq)
		if err != nil {
			log.Error().Err(err).Msg("")
		}
		tbSubnetReq.Name = v.IId.NameId
		tbSubnetReq.IdFromCsp = v.IId.SystemId

		_, err = CreateSubnet(nsId, content.Id, tbSubnetReq, true)
		if err != nil {
			log.Error().Err(err).Msg("")
		}
	}

	keyValue, err := kvstore.GetKv(Key)
	if err != nil {
		log.Error().Err(err).Msg("")
		err = fmt.Errorf("In CreateVNet(); kvstore.GetKv() returned an error.")
		log.Error().Err(err).Msg("")
		// return nil, err
	}

	result := model.TbVNetInfo{}
	err = json.Unmarshal([]byte(keyValue.Value), &result)
	if err != nil {
		log.Error().Err(err).Msg("")
	}

	// Store label info using CreateOrUpdateLabel
	labels := map[string]string{
		"provider":  "cb-tumblebug",
		"namespace": nsId,
	}
	err = label.CreateOrUpdateLabel(model.StrVNet, uuid, Key, labels)
	if err != nil {
		log.Error().Err(err).Msg("")
		return content, err
	}

	return result, nil
}
