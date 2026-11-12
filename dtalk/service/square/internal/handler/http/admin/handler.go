package admin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
	"github.com/web3chat/txchat/dtalk/service/square/internal/middleware"
	"github.com/web3chat/txchat/dtalk/service/square/internal/ossclient"
	"github.com/web3chat/txchat/dtalk/service/square/internal/pkg/httpresp"
	"github.com/web3chat/txchat/dtalk/service/square/internal/repository"
	"github.com/web3chat/txchat/dtalk/service/square/internal/usecase"
	"github.com/web3chat/txchat/dtalk/service/square/internal/xerror"
)

type Handler struct {
	svc *usecase.Service
}

func New(svc *usecase.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Captcha(c *gin.Context) {
	id, png, err := h.svc.NewCaptcha(c.Request.Context())
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	httpresp.AdminOK(c, gin.H{
		"captchaId": id,
		"image":     "data:image/png;base64," + png,
	})
}

func (h *Handler) Login(c *gin.Context) {
	var req struct {
		Username  string `json:"username"`
		Password  string `json:"password"`
		CaptchaID string `json:"captchaId"`
		Captcha   string `json:"captcha"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	token, admin, err := h.svc.AdminLogin(c.Request.Context(), req.Username, req.Password, req.CaptchaID, req.Captcha, c.ClientIP())
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	httpresp.AdminOK(c, gin.H{
		"token":       token,
		"username":    admin.Username,
		"role":        admin.Role,
		"permissions": admin.EffectivePerms(),
		"expire":      int(h.svc.Config().JWT.Expire.Seconds()),
	})
}

func (h *Handler) Logout(c *gin.Context) {
	cl := middleware.Admin(c)
	if cl == nil {
		httpresp.AdminErr(c, xerror.New(xerror.AdminUnauthorized, "未登录"))
		return
	}
	if err := h.svc.AdminLogout(c.Request.Context(), cl.ID, cl.Username); err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	h.audit(c, "logout", "", "", "")
	httpresp.AdminOK(c, gin.H{"username": cl.Username})
}

func (h *Handler) ChangePassword(c *gin.Context) {
	var req struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	cl := middleware.Admin(c)
	if err := h.svc.ChangeAdminPassword(c.Request.Context(), cl.AdminID, req.CurrentPassword, req.NewPassword); err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	h.audit(c, "change_password", "staff", strconv.FormatInt(cl.AdminID, 10), "")
	httpresp.AdminOK(c, gin.H{"username": cl.Username})
}

func (h *Handler) Me(c *gin.Context) {
	cl := middleware.Admin(c)
	httpresp.AdminOK(c, gin.H{
		"username":    cl.Username,
		"role":        cl.Role,
		"adminId":     cl.AdminID,
		"permissions": cl.Permissions,
	})
}

func (h *Handler) ListStaff(c *gin.Context) {
	list, catalog, err := h.svc.ListStaff(c.Request.Context())
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	httpresp.AdminOK(c, gin.H{"list": list, "catalog": catalog})
}

func (h *Handler) SetStaffStatus(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	var req struct {
		Status int16 `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	cl := middleware.Admin(c)
	if err := h.svc.SetStaffStatus(c.Request.Context(), cl.AdminID, id, req.Status); err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	h.audit(c, "set_staff_status", "staff", strconv.FormatInt(id, 10), strconv.Itoa(int(req.Status)))
	httpresp.AdminOK(c, gin.H{"id": id, "status": req.Status})
}

func (h *Handler) SetStaffPerms(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	var req struct {
		Permissions []string `json:"permissions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	cl := middleware.Admin(c)
	if err := h.svc.SetStaffPerms(c.Request.Context(), cl.AdminID, id, req.Permissions); err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	h.audit(c, "set_staff_permissions", "staff", strconv.FormatInt(id, 10), strings.Join(req.Permissions, ","))
	httpresp.AdminOK(c, gin.H{"id": id, "permissions": req.Permissions})
}

func (h *Handler) ListApply(c *gin.Context) {
	page := parsePage(c)
	list, total, err := h.svc.ListApply(c.Request.Context(), c.QueryArray("status"), page)
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	items := make([]gin.H, 0, len(list))
	for i := range list {
		items = append(items, applyJSON(&list[i]))
	}
	httpresp.AdminOK(c, gin.H{"list": items, "total": total, "page": page.Page, "pageSize": page.PageSize})
}

func (h *Handler) ReviewApply(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	var req struct {
		Approve bool   `json:"approve"`
		Remark  string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	cl := middleware.Admin(c)
	if err := h.svc.ReviewApply(c.Request.Context(), id, req.Approve, cl.Username, req.Remark); err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	h.audit(c, "review_apply", "apply", strconv.FormatInt(id, 10), boolStr(req.Approve))
	httpresp.AdminOK(c, gin.H{"id": id, "approve": req.Approve})
}

func (h *Handler) ListOfficial(c *gin.Context) {
	page := parsePage(c)
	order := parseOfficialOrder(c)
	list, total, err := h.svc.ListOfficial(c.Request.Context(), repository.OfficialListFilter{
		Statuses: parseInt16s(c.QueryArray("status")),
		Golds:    parseGoldFlags(c.QueryArray("gold")),
		Order:    order,
	}, page)
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	items := make([]gin.H, 0, len(list))
	for i := range list {
		items = append(items, officialJSON(&list[i]))
	}
	if order == "" {
		order = "created"
	}
	httpresp.AdminOK(c, gin.H{"list": items, "total": total, "page": page.Page, "pageSize": page.PageSize, "order": order})
}

func (h *Handler) BanOfficial(c *gin.Context) {
	addr := strings.TrimSpace(c.Param("addr"))
	if addr == "" {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	var req struct {
		Reason    string `json:"reason"`
		HidePosts bool   `json:"hidePosts"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	if err := h.svc.BanOfficial(c.Request.Context(), addr, req.Reason, req.HidePosts); err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	h.audit(c, "ban_official", "official", addr, req.Reason)
	httpresp.AdminOK(c, gin.H{"address": addr})
}

func (h *Handler) UnbanOfficial(c *gin.Context) {
	addr := strings.TrimSpace(c.Param("addr"))
	if addr == "" {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	if err := h.svc.UnbanOfficial(c.Request.Context(), addr); err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	h.audit(c, "unban_official", "official", addr, "")
	httpresp.AdminOK(c, gin.H{"address": addr})
}

func (h *Handler) RevokeOfficial(c *gin.Context) {
	addr := strings.TrimSpace(c.Param("addr"))
	if addr == "" {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	cl := middleware.Admin(c)
	if err := h.svc.RevokeOfficial(c.Request.Context(), addr, cl.Username); err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	h.audit(c, "revoke_official", "official", addr, "")
	httpresp.AdminOK(c, gin.H{"address": addr})
}

func (h *Handler) GoldOfficial(c *gin.Context) {
	h.setOfficialGold(c, true)
}

func (h *Handler) UngoldOfficial(c *gin.Context) {
	h.setOfficialGold(c, false)
}

func (h *Handler) setOfficialGold(c *gin.Context, gold bool) {
	addr := strings.TrimSpace(c.Param("addr"))
	if addr == "" {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	cl := middleware.Admin(c)
	if err := h.svc.SetOfficialGold(c.Request.Context(), addr, cl.Username, gold); err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	action := "ungold_official"
	if gold {
		action = "gold_official"
	}
	h.audit(c, action, "official", addr, "")
	httpresp.AdminOK(c, gin.H{"address": addr, "gold": gold})
}

func (h *Handler) PlatformOfficial(c *gin.Context) {
	oa, err := h.svc.PlatformOfficial(c.Request.Context())
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	names, err := h.svc.ListPlatformOfficialNames(c.Request.Context())
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	httpresp.AdminOK(c, gin.H{
		"address": oa.Address, "name": oa.Name, "avatar": oa.Avatar, "intro": oa.Intro,
		"names": officialNamesJSON(names),
	})
}

func (h *Handler) CreateOfficialName(c *gin.Context) {
	var req struct {
		Name   string `json:"name"`
		Avatar string `json:"avatar"`
		Intro  string `json:"intro"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	row, err := h.svc.CreateOfficialName(c.Request.Context(), req.Name, req.Avatar, req.Intro)
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	h.audit(c, "create_official_name", "official", entity.PlatformOfficialAddr, row.Name)
	httpresp.AdminOK(c, officialNameJSON(row))
}

func (h *Handler) UpdateOfficialName(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	var req struct {
		Name   string  `json:"name"`
		Avatar *string `json:"avatar"`
		Intro  *string `json:"intro"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	row, err := h.svc.UpdateOfficialName(c.Request.Context(), id, req.Name, req.Avatar, req.Intro)
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	h.audit(c, "update_official_name", "official", entity.PlatformOfficialAddr, row.Name)
	httpresp.AdminOK(c, officialNameJSON(row))
}

func (h *Handler) DeleteOfficialName(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	if err := h.svc.DeleteOfficialName(c.Request.Context(), id); err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	h.audit(c, "delete_official_name", "official", entity.PlatformOfficialAddr, c.Param("id"))
	httpresp.AdminOK(c, gin.H{"id": id})
}

func (h *Handler) UpdatePlatformOfficial(c *gin.Context) {
	var req struct {
		Name   string `json:"name"`
		Avatar string `json:"avatar"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	oa, err := h.svc.UpdatePlatformOfficial(c.Request.Context(), req.Name, req.Avatar)
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	h.audit(c, "update_platform_official", "official", oa.Address, oa.Name)
	httpresp.AdminOK(c, gin.H{
		"address": oa.Address, "name": oa.Name, "avatar": oa.Avatar, "intro": oa.Intro,
	})
}

func (h *Handler) Upload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, ossclient.MaxBytes+512<<10)
	fh, err := c.FormFile("file")
	if err != nil {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "请选择图片"))
		return
	}
	if fh.Size <= 0 {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "请选择图片"))
		return
	}
	if fh.Size > ossclient.MaxBytes {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "单张图片不超过 2MB"))
		return
	}
	f, err := fh.Open()
	if err != nil {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "读取图片失败"))
		return
	}
	defer f.Close()
	kind := strings.TrimSpace(c.PostForm("kind"))
	if kind == "" {
		kind = strings.TrimSpace(c.Query("kind"))
	}
	url, err := h.svc.AdminUploadImage(c.Request.Context(), kind, fh.Filename, fh.Size, f)
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	httpresp.AdminOK(c, gin.H{"url": url})
}

func (h *Handler) CreateOfficialPost(c *gin.Context) {
	req, ok := bindOfficialPost(c)
	if !ok {
		return
	}
	in, err := req.input()
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	p, err := h.svc.AdminCreatePost(c.Request.Context(), in)
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	h.audit(c, officialPostAudit(in.Draft, in.PublishAt != nil), "post", strconv.FormatInt(p.ID, 10), scheduleAuditExtra(in))
	httpresp.AdminOK(c, adminPostJSON(p, nil, false))
}

func (h *Handler) UpdateOfficialPost(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	req, ok := bindOfficialPost(c)
	if !ok {
		return
	}
	in, err := req.input()
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	p, err := h.svc.AdminUpdateOfficialPost(c.Request.Context(), id, in)
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	h.audit(c, officialPostAudit(in.Draft, in.PublishAt != nil), "post", strconv.FormatInt(p.ID, 10), scheduleAuditExtra(in))
	httpresp.AdminOK(c, adminPostJSON(p, nil, false))
}

func (h *Handler) PinPost(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	p, err := h.svc.AdminPinPost(c.Request.Context(), id)
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	h.audit(c, "pin_post", "post", strconv.FormatInt(id, 10), "")
	banned := h.svc.UserBanSet(c.Request.Context(), []string{p.AuthorAddr})
	httpresp.AdminOK(c, adminPostJSON(p, nil, banned[p.AuthorAddr]))
}

func (h *Handler) UnpinPost(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	p, err := h.svc.AdminUnpinPost(c.Request.Context(), id)
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	h.audit(c, "unpin_post", "post", strconv.FormatInt(id, 10), "")
	banned := h.svc.UserBanSet(c.Request.Context(), []string{p.AuthorAddr})
	httpresp.AdminOK(c, adminPostJSON(p, nil, banned[p.AuthorAddr]))
}

func (h *Handler) ListUserBans(c *gin.Context) {
	page := parsePage(c)
	addrs := parseTrimmed(c.QueryArray("address"))
	for i, a := range addrs {
		addrs[i] = strings.Join(strings.Fields(a), "")
	}
	list, total, err := h.svc.ListUserBans(c.Request.Context(), addrs, parseOrder(c), page)
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	items := make([]gin.H, 0, len(list))
	for i := range list {
		items = append(items, userBanJSON(&list[i]))
	}
	httpresp.AdminOK(c, gin.H{"list": items, "total": total, "page": page.Page, "pageSize": page.PageSize})
}

func (h *Handler) BanUser(c *gin.Context) {
	var req struct {
		Address      string `json:"address"`
		Reason       string `json:"reason"`
		HidePosts    bool   `json:"hidePosts"`
		HideComments bool   `json:"hideComments"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	addr := strings.TrimSpace(c.Param("addr"))
	if addr == "" {
		addr = strings.TrimSpace(req.Address)
	}
	if addr == "" {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "请填写钱包地址"))
		return
	}
	cl := middleware.Admin(c)
	if err := h.svc.BanUser(c.Request.Context(), addr, req.Reason, cl.Username, req.HidePosts, req.HideComments); err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	h.audit(c, "ban_user", "user", addr, usecase.BanAuditExtra(req.Reason, req.HidePosts, req.HideComments))
	httpresp.AdminOK(c, gin.H{"address": addr})
}

func (h *Handler) UnbanUser(c *gin.Context) {
	addr := strings.TrimSpace(c.Param("addr"))
	if addr == "" {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	cl := middleware.Admin(c)
	if err := h.svc.UnbanUser(c.Request.Context(), addr, cl.Username); err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	h.audit(c, "unban_user", "user", addr, "")
	httpresp.AdminOK(c, gin.H{"address": addr})
}

func (h *Handler) ListPosts(c *gin.Context) {
	page := parsePage(c)
	f := repository.PostListFilter{
		Statuses:        parseInt16s(c.QueryArray("status")),
		AuthorAddrs:     parseTrimmed(c.QueryArray("authorAddr")),
		AuthorNames:     parseTrimmed(c.QueryArray("authorName")),
		AuthorNameExact: parseTrimmed(c.QueryArray("authorNameExact")),
		Verdicts:        c.QueryArray("verdict"),
		Order:           parseOrder(c),
		GoldOnly:        goldOnly(c.QueryArray("gold")),
	}
	if v := c.Query("kind"); v != "" {
		n, _ := strconv.ParseInt(v, 10, 16)
		k := int16(n)
		f.Kind = &k
	}
	f.Tags = parseTrimmed(c.QueryArray("tag"))
	if t := parseTime(c.Query("from")); t != nil {
		f.From = t
	}
	if t := parseTime(c.Query("to")); t != nil {
		f.To = t
	}
	if usecase.IsRankOrder(f.Order) && f.From == nil {
		f.From = usecase.RankWindowStart(parseRankWindow(c), time.Now())
	}
	list, total, err := h.svc.AdminListPosts(c.Request.Context(), f, page)
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	ids := make([]int64, 0, len(list))
	for i := range list {
		ids = append(ids, list[i].ID)
	}
	mods, _ := h.svc.LatestModerationMap(c.Request.Context(), entity.TargetPost, ids)
	banned := h.svc.UserBanSet(c.Request.Context(), postAuthorAddrs(list))
	items := make([]gin.H, 0, len(list))
	for i := range list {
		items = append(items, adminPostJSON(&list[i], mods[list[i].ID], banned[list[i].AuthorAddr]))
	}
	httpresp.AdminOK(c, gin.H{"list": items, "total": total, "page": page.Page, "pageSize": page.PageSize})
}

func (h *Handler) ListOfficialPostReplies(c *gin.Context) {
	page := parsePage(c)
	list, total, err := h.svc.AdminListOfficialPostReplies(c.Request.Context(), page)
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	items := make([]gin.H, 0, len(list))
	for i := range list {
		items = append(items, officialPostReplyJSON(&list[i]))
	}
	httpresp.AdminOK(c, gin.H{"list": items, "total": total, "page": page.Page, "pageSize": page.PageSize})
}

func (h *Handler) GetPost(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	p, err := h.svc.AdminGetPost(c.Request.Context(), id)
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	mods, _ := h.svc.LatestModerationMap(c.Request.Context(), entity.TargetPost, []int64{p.ID})
	banned := h.svc.UserBanSet(c.Request.Context(), []string{p.AuthorAddr})
	httpresp.AdminOK(c, adminPostJSON(p, mods[p.ID], banned[p.AuthorAddr]))
}

func (h *Handler) SetPostStatus(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	var req struct {
		Status int16 `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	if !validContentStatus(req.Status) {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "状态不合法"))
		return
	}
	if err := h.svc.AdminSetPostStatus(c.Request.Context(), id, req.Status); err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	h.audit(c, "set_post_status", "post", strconv.FormatInt(id, 10), strconv.Itoa(int(req.Status)))
	httpresp.AdminOK(c, gin.H{"id": id, "status": req.Status})
}

func (h *Handler) BatchDeletePosts(c *gin.Context) {
	var req struct {
		IDs []int64 `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil && c.Request.ContentLength > 0 {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	ids := append([]int64{}, req.IDs...)
	ids = append(ids, parseInt64s(c.QueryArray("id"))...)
	ids = append(ids, parseInt64s(c.QueryArray("ids"))...)
	updated, err := h.svc.AdminBatchDeletePosts(c.Request.Context(), ids)
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	for _, id := range updated {
		h.audit(c, "set_post_status", "post", strconv.FormatInt(id, 10), strconv.Itoa(int(entity.StatusDeleted)))
	}
	httpresp.AdminOK(c, gin.H{"deleted": len(updated), "ids": updated})
}

func (h *Handler) OverridePostAI(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	var req struct {
		Pass bool `json:"pass"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	cl := middleware.Admin(c)
	if err := h.svc.OverridePostAI(c.Request.Context(), id, cl.Username, req.Pass); err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	h.audit(c, "override_post_ai", "post", strconv.FormatInt(id, 10), boolStr(req.Pass))
	httpresp.AdminOK(c, gin.H{"id": id, "pass": req.Pass})
}

func (h *Handler) ListComments(c *gin.Context) {
	page := parsePage(c)
	f := repository.CommentListFilter{
		AuthorAddr: strings.TrimSpace(c.Query("authorAddr")),
		Statuses:   parseInt16s(c.QueryArray("status")),
		Verdicts:   c.QueryArray("verdict"),
		Order:      parseOrder(c),
	}
	if v := c.Query("postId"); v != "" {
		n, _ := strconv.ParseInt(v, 10, 64)
		f.PostID = n
	}
	list, total, err := h.svc.AdminListComments(c.Request.Context(), f, page)
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	ids := make([]int64, 0, len(list))
	for i := range list {
		ids = append(ids, list[i].ID)
	}
	mods, _ := h.svc.LatestModerationMap(c.Request.Context(), entity.TargetComment, ids)
	banned := h.svc.UserBanSet(c.Request.Context(), commentAuthorAddrs(list))
	items := make([]gin.H, 0, len(list))
	for i := range list {
		items = append(items, adminCommentJSON(&list[i], mods[list[i].ID], banned[list[i].AuthorAddr]))
	}
	httpresp.AdminOK(c, gin.H{"list": items, "total": total, "page": page.Page, "pageSize": page.PageSize})
}

func (h *Handler) SetCommentStatus(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	var req struct {
		Status int16 `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	if !validContentStatus(req.Status) {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "状态不合法"))
		return
	}
	if err := h.svc.AdminSetCommentStatus(c.Request.Context(), id, req.Status); err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	h.audit(c, "set_comment_status", "comment", strconv.FormatInt(id, 10), strconv.Itoa(int(req.Status)))
	httpresp.AdminOK(c, gin.H{"id": id, "status": req.Status})
}

func (h *Handler) OverrideCommentAI(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	var req struct {
		Pass bool `json:"pass"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	cl := middleware.Admin(c)
	if err := h.svc.OverrideCommentAI(c.Request.Context(), id, cl.Username, req.Pass); err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	h.audit(c, "override_comment_ai", "comment", strconv.FormatInt(id, 10), boolStr(req.Pass))
	httpresp.AdminOK(c, gin.H{"id": id, "pass": req.Pass})
}

func (h *Handler) CreateOfficialComment(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	var req struct {
		Content   string `json:"content"`
		ReplyToID int64  `json:"replyToId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	cl := middleware.Admin(c)
	cm, err := h.svc.AdminCreateOfficialComment(c.Request.Context(), id, req.Content, cl.Username, req.ReplyToID)
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	h.audit(c, "official_reply", "post", strconv.FormatInt(id, 10), strconv.FormatInt(cm.ID, 10))
	httpresp.AdminOK(c, adminCommentJSON(cm, nil, false))
}

func (h *Handler) Overview(c *gin.Context) {
	days := parseDays(c)
	addrs := parseTrimmed(c.QueryArray("authorAddr"))
	stats, err := h.svc.AdminOverview(c.Request.Context(), days, addrs)
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	series := make([]gin.H, 0, len(stats.Series))
	for _, d := range stats.Series {
		series = append(series, statDayJSON(d))
	}
	authorSeries := make([]gin.H, 0, len(stats.AuthorSeries))
	for _, d := range stats.AuthorSeries {
		authorSeries = append(authorSeries, statDayJSON(d))
	}
	httpresp.AdminOK(c, gin.H{
		"timezone":     stats.Timezone,
		"days":         stats.Days,
		"today":        statCountsJSON(stats.Today),
		"yesterday":    statCountsJSON(stats.Yesterday),
		"total":        statCountsJSON(stats.Total),
		"series":       series,
		"authorAddrs":  stats.AuthorAddrs,
		"authorTotal":  stats.AuthorTotal,
		"authorSeries": authorSeries,
	})
}

func (h *Handler) ListAccounts(c *gin.Context) {
	page := parsePage(c)
	list, total, err := h.svc.AdminListAccounts(c.Request.Context(), parseTrimmed(c.QueryArray("address")), parseTrimmed(c.QueryArray("authorName")), page)
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	items := make([]gin.H, 0, len(list))
	for i := range list {
		a := &list[i]
		var joined any
		if a.JoinedAt != nil {
			joined = a.JoinedAt.Format(time.RFC3339)
		}
		items = append(items, gin.H{
			"address": a.Address, "authorName": a.AuthorName, "isOfficial": a.IsOfficial,
			"officialStatus": a.OfficialStatus, "postCount": a.PostCount,
			"followerCount": a.FollowerCount, "followingCount": a.FollowingCount,
			"avatar": a.Avatar, "intro": a.Intro, "gold": a.Gold,
			"officialBanReason": a.OfficialBanReason, "reportCount": a.ReportCount,
			"userBanned": a.UserBanned, "userBanReason": a.UserBanReason,
			"joinedAt": joined,
		})
	}
	httpresp.AdminOK(c, gin.H{"list": items, "total": total, "page": page.Page, "pageSize": page.PageSize})
}

func (h *Handler) ListAuthorRanks(c *gin.Context) {
	page := parsePage(c)
	order := parseAuthorRankOrder(c)
	window := parseRankWindow(c)
	f := repository.AuthorRankFilter{
		Order:       order,
		From:        usecase.RankWindowStart(window, time.Now()),
		AuthorAddrs: parseTrimmed(append(c.QueryArray("authorAddr"), c.QueryArray("address")...)),
		AuthorNames: parseTrimmed(c.QueryArray("authorName")),
	}
	list, total, err := h.svc.AdminListAuthorRanks(c.Request.Context(), f, page)
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	rankBase := page.Offset()
	items := make([]gin.H, 0, len(list))
	for i := range list {
		a := &list[i]
		items = append(items, gin.H{
			"rank": rankBase + i + 1, "address": a.Address, "authorName": a.AuthorName,
			"authorKind": a.AuthorKind, "authorAvatar": a.AuthorAvatar, "isOfficial": a.IsOfficial,
			"postCount": a.PostCount, "likeCount": a.LikeCount, "viewCount": a.ViewCount,
			"commentCount": a.CommentCount,
		})
	}
	httpresp.AdminOK(c, gin.H{
		"list": items, "total": total, "page": page.Page, "pageSize": page.PageSize,
		"order": order, "window": window,
	})
}

func (h *Handler) ListReports(c *gin.Context) {
	page := parsePage(c)
	f := repository.ReportListFilter{
		Statuses:      parseInt16s(c.QueryArray("status")),
		Reasons:       parseTrimmed(c.QueryArray("reason")),
		ReporterAddrs: parseTrimmed(c.QueryArray("reporterAddr")),
		TargetIDs:     parseInt64s(c.QueryArray("targetId")),
	}
	list, total, err := h.svc.AdminListReports(c.Request.Context(), f, page)
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	items := make([]gin.H, 0, len(list))
	for i := range list {
		items = append(items, reportJSON(&list[i]))
	}
	httpresp.AdminOK(c, gin.H{"list": items, "total": total, "page": page.Page, "pageSize": page.PageSize})
}

func (h *Handler) HandleReport(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	var req struct {
		Action string `json:"action"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	cl := middleware.Admin(c)
	if err := h.svc.AdminHandleReport(c.Request.Context(), id, req.Action, cl.Username); err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	h.audit(c, "handle_report", "report", strconv.FormatInt(id, 10), req.Action)
	httpresp.AdminOK(c, gin.H{"id": id, "action": req.Action})
}

func (h *Handler) GetSettings(c *gin.Context) {
	st, err := h.svc.AdminSquareSettings(c.Request.Context())
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	httpresp.AdminOK(c, settingsJSON(st))
}

func (h *Handler) UpdateSettings(c *gin.Context) {
	var req struct {
		GateMode        string `json:"gateMode"`
		GateMessage     string `json:"gateMessage"`
		AllowOfficials  *bool  `json:"allowOfficials"`
		ModerationLevel string `json:"moderationLevel"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	in := entity.SquareSettings{
		GateMode:        req.GateMode,
		GateMessage:     req.GateMessage,
		AllowOfficials:  true,
		ModerationLevel: req.ModerationLevel,
	}
	if req.AllowOfficials != nil {
		in.AllowOfficials = *req.AllowOfficials
	}
	cl := middleware.Admin(c)
	st, err := h.svc.UpdateSquareSettings(c.Request.Context(), in, cl.Username)
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	h.audit(c, "update_square_settings", "settings", st.GateMode, st.GateMode+" / "+st.ModerationLevel)
	httpresp.AdminOK(c, settingsJSON(st))
}

func (h *Handler) AddPosterAllow(c *gin.Context) {
	var req struct {
		Address   string   `json:"address"`
		Addresses []string `json:"addresses"`
		Remark    string   `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	addrs := append([]string{}, req.Addresses...)
	if strings.TrimSpace(req.Address) != "" {
		addrs = append(addrs, req.Address)
	}
	addrs = append(addrs, parseTrimmedAdmin(c.QueryArray("address"))...)
	if len(addrs) == 0 {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "请填写钱包地址"))
		return
	}
	cl := middleware.Admin(c)
	for _, addr := range addrs {
		if err := h.svc.AddPosterAllow(c.Request.Context(), addr, req.Remark, cl.Username); err != nil {
			httpresp.AdminErr(c, err)
			return
		}
		h.audit(c, "add_poster_allow", "user", strings.TrimSpace(addr), req.Remark)
	}
	st, err := h.svc.AdminSquareSettings(c.Request.Context())
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	httpresp.AdminOK(c, settingsJSON(st))
}

func (h *Handler) RemovePosterAllow(c *gin.Context) {
	addrs := parseTrimmedAdmin(append([]string{c.Param("addr")}, c.QueryArray("address")...))
	if len(addrs) == 0 {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "请填写钱包地址"))
		return
	}
	cl := middleware.Admin(c)
	for _, addr := range addrs {
		if err := h.svc.RemovePosterAllow(c.Request.Context(), addr, cl.Username); err != nil {
			httpresp.AdminErr(c, err)
			return
		}
		h.audit(c, "remove_poster_allow", "user", addr, "")
	}
	st, err := h.svc.AdminSquareSettings(c.Request.Context())
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	httpresp.AdminOK(c, settingsJSON(st))
}

func parseTrimmedAdmin(vals []string) []string {
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func settingsJSON(st entity.SquareSettings) gin.H {
	posters := make([]gin.H, 0, len(st.Posters))
	for _, p := range st.Posters {
		posters = append(posters, gin.H{
			"address": p.Address, "remark": p.Remark, "operator": p.Operator,
			"createdAt": p.CreatedAt.Format(time.RFC3339),
		})
	}
	out := gin.H{
		"gateMode":        st.GateMode,
		"gateMessage":     st.GateMessage,
		"allowOfficials":  st.AllowOfficials,
		"moderationLevel": st.ModerationLevel,
		"posters":         posters,
	}
	if !st.UpdatedAt.IsZero() {
		out["updatedAt"] = st.UpdatedAt.Format(time.RFC3339)
	}
	return out
}

func (h *Handler) LoginAlert(c *gin.Context) {
	sessionID := ""
	if cl := middleware.Admin(c); cl != nil {
		sessionID = cl.ID
	}
	alert, err := h.svc.LoginAlert(c.Request.Context(), c.ClientIP(), sessionID)
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	ips := make([]gin.H, 0, len(alert.IPs))
	for _, row := range alert.IPs {
		ips = append(ips, gin.H{
			"ip": row.IP, "fails": row.Fails, "lastAt": row.LastAt.Format(time.RFC3339),
		})
	}
	shares := make([]gin.H, 0, len(alert.Shares))
	for _, sh := range alert.Shares {
		shareIPs := make([]gin.H, 0, len(sh.IPs))
		for _, ip := range sh.IPs {
			shareIPs = append(shareIPs, gin.H{
				"ip": ip.IP, "current": ip.Current, "lastSeen": ip.LastSeen.Format(time.RFC3339),
			})
		}
		shares = append(shares, gin.H{"username": sh.Username, "ipCount": sh.IPCount, "ips": shareIPs})
	}
	accounts := make([]gin.H, 0, len(alert.Accounts))
	for _, row := range alert.Accounts {
		sessions := make([]gin.H, 0, len(row.Sessions))
		for _, sess := range row.Sessions {
			sessions = append(sessions, gin.H{
				"ip": sess.IP, "current": sess.Current, "lastSeen": sess.LastSeen.Format(time.RFC3339),
			})
		}
		accounts = append(accounts, gin.H{
			"username": row.Username, "sessionCount": row.SessionCount, "sessions": sessions,
		})
	}
	httpresp.AdminOK(c, gin.H{
		"alert": alert.Alert, "windowMin": int(alert.Window.Minutes()), "minFails": alert.MinFails,
		"ipCount": alert.IPCount, "failCount": alert.FailCount, "ips": ips,
		"shareAlert": alert.ShareAlert, "shares": shares,
		"multiAlert": alert.MultiAlert, "accounts": accounts,
	})
}

func (h *Handler) ListAudit(c *gin.Context) {
	page := parsePage(c)
	f := repository.AuditFilter{
		Actions:     c.QueryArray("action"),
		TargetTypes: c.QueryArray("targetType"),
		TargetID:    strings.TrimSpace(c.Query("targetId")),
	}
	list, total, err := h.svc.ListAudit(c.Request.Context(), f, page)
	if err != nil {
		httpresp.AdminErr(c, err)
		return
	}
	items := make([]gin.H, 0, len(list))
	for i := range list {
		a := &list[i]
		items = append(items, gin.H{
			"id": a.ID, "adminId": a.AdminID, "username": a.Username, "action": a.Action,
			"targetType": a.TargetType, "targetId": a.TargetID, "extra": a.Extra,
			"clientIp": a.ClientIP, "createdAt": a.CreatedAt.Format(time.RFC3339),
		})
	}
	httpresp.AdminOK(c, gin.H{"list": items, "total": total, "page": page.Page, "pageSize": page.PageSize})
}

func (h *Handler) audit(c *gin.Context, action, targetType, targetID, extra string) {
	cl := middleware.Admin(c)
	h.svc.Audit(c.Request.Context(), &entity.Audit{
		AdminID: cl.AdminID, Username: cl.Username, Action: action,
		TargetType: targetType, TargetID: targetID, Extra: extra, ClientIP: c.ClientIP(),
	})
}

func parsePage(c *gin.Context) entity.PageQuery {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	return entity.PageQuery{Page: page, PageSize: size}.Normalize()
}

func parseID(s string) (int64, error) {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil || id <= 0 {
		return 0, xerror.New(xerror.ParamInvalid, "参数错误")
	}
	return id, nil
}

type officialPostReq struct {
	Title           string   `json:"title"`
	Intro           string   `json:"intro"`
	Content         string   `json:"content"`
	MediaURLs       []string `json:"mediaUrls"`
	CommentsEnabled *bool    `json:"commentsEnabled"`
	Name            string   `json:"name"`
	Avatar          string   `json:"avatar"`
	Draft           bool     `json:"draft"`
	Tag             string   `json:"tag"`
	PublishAt       string   `json:"publishAt"`
	ExpireAfterSec  int64    `json:"expireAfterSec"`
}

func bindOfficialPost(c *gin.Context) (officialPostReq, bool) {
	var req officialPostReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.AdminErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return officialPostReq{}, false
	}
	return req, true
}

func (req officialPostReq) input() (usecase.AdminCreatePostInput, error) {
	var publishAt *time.Time
	if raw := strings.TrimSpace(req.PublishAt); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return usecase.AdminCreatePostInput{}, xerror.New(xerror.ParamInvalid, "发布时间格式错误")
		}
		publishAt = &t
	}
	return usecase.AdminCreatePostInput{
		Title:           req.Title,
		Intro:           req.Intro,
		Content:         req.Content,
		MediaURLs:       req.MediaURLs,
		CommentsEnabled: req.CommentsEnabled,
		Name:            req.Name,
		Avatar:          req.Avatar,
		Draft:           req.Draft,
		Tag:             req.Tag,
		PublishAt:       publishAt,
		ExpireAfterSec:  req.ExpireAfterSec,
	}, nil
}

func officialPostAudit(draft, scheduled bool) string {
	if scheduled {
		return "schedule_official_post"
	}
	if draft {
		return "save_official_draft"
	}
	return "create_official_post"
}

func scheduleAuditExtra(in usecase.AdminCreatePostInput) string {
	if in.PublishAt != nil {
		return in.PublishAt.Format(time.RFC3339)
	}
	if in.ExpireAfterSec > 0 {
		return strconv.FormatInt(in.ExpireAfterSec, 10)
	}
	return ""
}

func parseGoldFlags(vals []string) []bool {
	seen := map[bool]struct{}{}
	out := make([]bool, 0, 2)
	for _, v := range vals {
		var b bool
		ok := true
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "yes":
			b = true
		case "0", "false", "no":
			b = false
		default:
			ok = false
		}
		if !ok {
			continue
		}
		if _, dup := seen[b]; dup {
			continue
		}
		seen[b] = struct{}{}
		out = append(out, b)
	}
	return out
}

func goldOnly(vals []string) bool {
	for _, v := range parseGoldFlags(vals) {
		if v {
			return true
		}
	}
	return false
}

func parseInt16s(vals []string) []int16 {
	out := make([]int16, 0, len(vals))
	for _, v := range vals {
		n, err := strconv.ParseInt(v, 10, 16)
		if err != nil {
			continue
		}
		out = append(out, int16(n))
	}
	return out
}

func parseInt64s(vals []string) []int64 {
	out := make([]int64, 0, len(vals))
	for _, v := range vals {
		n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err != nil || n <= 0 {
			continue
		}
		out = append(out, n)
	}
	return out
}

func parseTrimmed(vals []string) []string {
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func parseDays(c *gin.Context) int {
	days := 14
	for _, v := range c.QueryArray("days") {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || n <= 0 {
			continue
		}
		days = n
	}
	return days
}

func parseOfficialOrder(c *gin.Context) string {
	switch strings.ToLower(strings.TrimSpace(c.Query("order"))) {
	case "followers", "posts", "reports":
		return strings.ToLower(strings.TrimSpace(c.Query("order")))
	default:
		return ""
	}
}

func parseOrder(c *gin.Context) string {
	for _, v := range c.QueryArray("order") {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "asc", "desc", "hot", "likes", "comments", "followers", "posts":
			return strings.ToLower(strings.TrimSpace(v))
		}
	}
	return ""
}

func parseAuthorRankOrder(c *gin.Context) string {
	for _, v := range c.QueryArray("order") {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "likes", "views", "view":
			if strings.EqualFold(strings.TrimSpace(v), "view") {
				return "views"
			}
			return strings.ToLower(strings.TrimSpace(v))
		}
	}
	return "likes"
}

func parseRankWindow(c *gin.Context) string {
	for _, v := range c.QueryArray("window") {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "today", "week", "month":
			return strings.ToLower(strings.TrimSpace(v))
		}
	}
	days := 0
	for _, v := range c.QueryArray("hotDays") {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || n <= 0 {
			continue
		}
		days = n
	}
	if days <= 0 {
		return "week"
	}
	if days <= 1 {
		return "today"
	}
	if days >= 30 {
		return "month"
	}
	return "week"
}

func parseTime(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}

func validContentStatus(st int16) bool {
	switch st {
	case entity.StatusVisible, entity.StatusHidden, entity.StatusDeleted, entity.StatusAIReject, entity.StatusPending:
		return true
	default:
		return false
	}
}

func boolStr(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func statCountsJSON(c entity.StatCounts) gin.H {
	return gin.H{"posts": c.Posts, "comments": c.Comments, "views": c.Views, "likes": c.Likes}
}

func statDayJSON(d entity.StatDay) gin.H {
	return gin.H{"date": d.Date, "posts": d.Posts, "comments": d.Comments, "views": d.Views, "likes": d.Likes}
}

func reportJSON(r *entity.Report) gin.H {
	var handled any
	if r.HandledAt != nil {
		handled = r.HandledAt.Format(time.RFC3339)
	}
	return gin.H{
		"id": r.ID, "reporterAddr": r.ReporterAddr, "targetType": r.TargetType, "targetId": r.TargetID,
		"reason": r.Reason, "detail": r.Detail, "status": r.Status, "handledBy": r.HandledBy,
		"handledAt": handled, "createdAt": r.CreatedAt.Format(time.RFC3339),
		"title": r.PostTitle, "intro": r.PostIntro, "authorAddr": r.AuthorAddr,
		"authorName": r.AuthorName, "authorKind": r.AuthorKind, "postStatus": r.PostStatus,
	}
}

func applyJSON(a *entity.OfficialApply) gin.H {
	var reviewed any
	if a.ReviewedAt != nil {
		reviewed = a.ReviewedAt.Format(time.RFC3339)
	}
	return gin.H{
		"id": a.ID, "address": a.Address, "name": a.Name, "avatar": a.Avatar, "intro": a.Intro,
		"status": a.Status, "reviewer": a.Reviewer, "reviewRemark": a.ReviewRemark,
		"createdAt": a.CreatedAt.Format(time.RFC3339), "reviewedAt": reviewed,
	}
}

func userBanJSON(b *entity.UserBan) gin.H {
	return gin.H{
		"address": b.Address, "authorName": b.AuthorName, "reason": b.Reason, "operator": b.Operator,
		"createdAt": b.CreatedAt.Format(time.RFC3339), "updatedAt": b.UpdatedAt.Format(time.RFC3339),
	}
}

func postAuthorAddrs(list []entity.Post) []string {
	out := make([]string, 0, len(list))
	for i := range list {
		out = append(out, list[i].AuthorAddr)
	}
	return out
}

func commentAuthorAddrs(list []entity.Comment) []string {
	out := make([]string, 0, len(list))
	for i := range list {
		out = append(out, list[i].AuthorAddr)
	}
	return out
}

func officialNameJSON(n *entity.OfficialName) gin.H {
	if n == nil {
		return gin.H{}
	}
	return gin.H{"id": n.ID, "name": n.Name, "avatar": n.Avatar, "intro": n.Intro, "draftCount": n.DraftCount, "postCount": n.PostCount}
}

func officialNamesJSON(list []entity.OfficialName) []gin.H {
	out := make([]gin.H, 0, len(list))
	for i := range list {
		out = append(out, officialNameJSON(&list[i]))
	}
	return out
}

func officialJSON(o *entity.OfficialAccount) gin.H {
	var banned any
	if o.BannedAt != nil {
		banned = o.BannedAt.Format(time.RFC3339)
	}
	var goldAt any
	if o.GoldAt != nil {
		goldAt = o.GoldAt.Format(time.RFC3339)
	}
	return gin.H{
		"address": o.Address, "name": o.Name, "avatar": o.Avatar, "intro": o.Intro,
		"status": o.Status, "applyId": o.ApplyID, "bannedAt": banned, "banReason": o.BanReason,
		"gold": o.Gold, "goldAt": goldAt,
		"createdAt": o.CreatedAt.Format(time.RFC3339),
		"postCount": o.PostCount, "followerCount": o.FollowerCount, "reportCount": o.ReportCount,
	}
}

func officialPostReplyJSON(row *entity.OfficialPostReply) gin.H {
	return gin.H{
		"postId": row.PostID, "postTitle": row.PostTitle, "postStatus": row.PostStatus,
		"replyCount": row.ReplyCount, "latestCommentId": row.LatestCommentID,
		"latestAuthorName": row.LatestAuthorName, "latestAuthorAddr": row.LatestAuthorAddr,
		"latestContent": row.LatestContent, "latestAt": row.LatestAt.Format(time.RFC3339),
	}
}

func adminPostJSON(p *entity.Post, m *entity.Moderation, authorBanned bool) gin.H {
	var media []string
	_ = json.Unmarshal(p.MediaJSON, &media)
	if media == nil {
		media = []string{}
	}
	out := gin.H{
		"id": p.ID, "authorAddr": p.AuthorAddr, "authorName": p.AuthorName, "authorKind": p.AuthorKind,
		"authorAvatar": p.AuthorAvatar, "authorIntro": p.AuthorIntro,
		"title": p.Title, "intro": p.Intro, "tag": p.Tag,
		"content": p.Content, "contentFormat": p.ContentFormat(), "mediaUrls": media, "commentsEnabled": p.CommentsEnabled == 1,
		"status": p.Status, "commentCount": p.CommentCount, "likeCount": p.LikeCount, "viewCount": p.ViewCount,
		"pinned": p.Pinned(), "pinnedAt": pinnedAtJSON(p), "authorGold": p.AuthorGold,
		"publishAt": timePtrJSON(p.PublishAt), "expireAt": timePtrJSON(p.ExpireAt), "expireAfterSec": p.ExpireAfterSec,
		"createdAt": p.CreatedAt.Format(time.RFC3339), "updatedAt": p.UpdatedAt.Format(time.RFC3339), "authorBanned": authorBanned,
	}
	if m != nil {
		out["moderation"] = moderationJSON(m)
	}
	return out
}

func timePtrJSON(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Format(time.RFC3339)
}

func pinnedAtJSON(p *entity.Post) any {
	if p == nil || p.PinnedAt == nil {
		return nil
	}
	return p.PinnedAt.Format(time.RFC3339)
}

func adminCommentJSON(cm *entity.Comment, m *entity.Moderation, authorBanned bool) gin.H {
	out := gin.H{
		"id": cm.ID, "postId": cm.PostID, "authorAddr": cm.AuthorAddr, "authorName": cm.AuthorName,
		"content": cm.Content, "status": cm.Status, "floor": cm.Floor, "createdAt": cm.CreatedAt.Format(time.RFC3339),
		"authorBanned": authorBanned, "isOfficial": cm.AuthorAddr == entity.PlatformOfficialAddr,
		"parentId": cm.ParentID, "replyToId": cm.ReplyToID, "replyToAddr": cm.ReplyToAddr, "replyToName": cm.ReplyToName,
		"replyCount": cm.ReplyCount,
	}
	if m != nil {
		out["moderation"] = moderationJSON(m)
	}
	return out
}

func moderationJSON(m *entity.Moderation) gin.H {
	var cats []string
	_ = json.Unmarshal(m.Categories, &cats)
	if cats == nil {
		cats = []string{}
	}
	out := gin.H{
		"verdict": m.Verdict, "categories": cats, "reason": m.Reason, "model": m.Model, "latencyMs": m.LatencyMS,
	}
	if m.OverriddenBy != nil {
		out["overriddenBy"] = *m.OverriddenBy
	}
	if m.FinalVerdict != nil {
		out["finalVerdict"] = *m.FinalVerdict
	}
	return out
}
