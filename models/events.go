package models

type JoinRequest struct {
	Id        uint64 `json:"id"`
	IsdAs     string `json:"isdas"`
	AsToQuery string `json:"isdas"`
	SigKey    string `json:"sigkey"`
	EncKey    string `json:"enckey"`
}

func FindJoinRequestsByIsdAs(isdas string) ([]JoinRequest, error) {
	var requests []JoinRequest
	_, err := o.QueryTable("join_request").Filter("IsdAs", isdas).All(&requests)
	return requests, err
}

func (jr *JoinRequest) Insert() error {
	_, err := o.Insert(jr)
	return err
}

func DeleteJoinRequestById(id uint64) error {
	_, err := o.Delete(&JoinRequest{Id: id})
	return err
}

type JoinRequestMapping struct {
	Id      uint64
	IsdAs   string
	Account *Account `orm:"rel(fk)"`
}

func FindJoinMappingByRequestId(id uint64) (*JoinRequestMapping, error) {
	jrm := new(JoinRequestMapping)
	err := o.QueryTable(jrm).Filter("Id", id).RelatedSel().One(jrm)
	return jrm, err
}

func (jrm *JoinRequestMapping) Insert() error {
	_, err := o.Insert(jrm)
	return err
}

func (jrm *JoinRequestMapping) Delete() error {
	_, err := o.Delete(jrm)
	return err
}

type JoinReply struct {
	RequestId   uint64 `json:"request_id" orm:"pk"`
	IsdAs       string `json:"isdas"`
	Certificate string `json:"certificate" orm:"size(1000)"`
	TRC         string `json:"trc"`
}

func FindJoinReplyByRequestId(id uint64) (*JoinReply, error) {
	jr := new(JoinReply)
	err := o.QueryTable(jr).Filter("RequestId", id).RelatedSel().One(jr)
	return jr, err
}

func (jr *JoinReply) Insert() error {
	_, err := o.Insert(jr)
	return err
}

func (jr *JoinReply) Delete() error {
	_, err := o.Delete(jr)
	return err
}

type ConnRequest struct {
	Id                   uint64 `json:"id"`
	IsdAs                string `json:"isdas"`
	RequesterIsdAs       string `json:"requester_isdas"`
	RequesterCertificate string `json:"requester_certificate" orm:"size(1000)"`
	IP                   string `json:"ip"`
	Port                 uint64 `json:"port"`
	MTU                  uint64 `json:"mtu"`
	Bandwidth            uint64 `json:"bandwidth"`
	Linktype             string `json:"linktype"`
}

func FindConnRequestsByIsdAs(isdas string) ([]ConnRequest, error) {
	var requests []ConnRequest
	_, err := o.QueryTable("conn_request").Filter("IsdAs", isdas).All(&requests)
	return requests, err
}

func (cr *ConnRequest) Insert() error {
	_, err := o.Insert(cr)
	return err
}

func DeleteConnRequestById(id uint64) error {
	_, err := o.Delete(&ConnRequest{Id: id})
	return err
}

type ConnRequestMapping struct {
	RequestId      uint64 `orm:"pk"`
	RequesterIsdAs string
	ServerIsdAs    string
}

func FindConnMappingByRequestId(id uint64) (ConnRequestMapping, error) {
	var crm ConnRequestMapping
	err := o.QueryTable(crm).Filter("RequestId", id).RelatedSel().One(&crm)
	return crm, err
}

func (crm *ConnRequestMapping) Insert() error {
	_, err := o.Insert(crm)
	return err
}

func DeleteConnMappingById(id uint64) error {
	_, err := o.Delete(&ConnRequestMapping{RequestId: id})
	return err
}

type ConnReply struct {
	RequestId      uint64 `json:"request_id" orm:"pk"`
	RequesterIsdAs string `json:"requester_isdas"`
	Certificate    string `json:"certificate" orm:"size(1000)"`
	IP             string `json:"ip"`
	Port           uint64 `json:"port"`
	MTU            uint64 `json:"mtu"`
	Bandwidth      uint64 `json:"bandwidth"`
}

func FindConnRepliesByIsdAs(isdas string) ([]ConnReply, error) {
	var cr []ConnReply
	_, err := o.QueryTable("conn_reply").Filter("RequesterIsdAs", isdas).RelatedSel().All(&cr)
	return cr, err
}

func (cr *ConnReply) Insert() error {
	_, err := o.Insert(cr)
	return err
}

func DeleteConnReplyById(id uint64) error {
	_, err := o.Delete(&ConnReply{RequestId: id})
	return err
}
