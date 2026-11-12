package entity

import (
	"strings"
	"time"
)

const (
	StatusVisible  int16 = 1
	StatusHidden   int16 = 2
	StatusDeleted  int16 = 3
	StatusAIReject int16 = 4
	StatusPending  int16 = 5
	StatusDraft    int16 = 6

	KindPersonal int16 = 0
	KindOfficial int16 = 1

	// OfficialPostTag* 平台官方帖标签（运营台可选，App 可按标签筛选）。
	OfficialPostTagAnnouncement = "公告"
	OfficialPostTagMessage      = "消息"
	OfficialPostTagActivity     = "活动"

	OAStatusNormal int16 = 1
	OAStatusBanned int16 = 2

	ApplyPending  = "pending"
	ApplyApproved = "approved"
	ApplyRejected = "rejected"

	VerdictPass   = "pass"
	VerdictReject = "reject"
	VerdictReview = "review"

	TargetPost    = "post"
	TargetComment = "comment"

	RoleSuper    = "super"
	RoleReviewer = "reviewer"

	ActionPost          = "post"
	ActionComment       = "comment"
	ActionLike          = "like"
	ActionOfficialApply = "official_apply"
	ActionFollow        = "follow"
	ActionReport        = "report"
	ActionEdit          = "edit"

	MaxPinnedPosts = 3

	FeedLatest    = "latest"
	FeedHot       = "hot"
	FeedFollowing = "following"

	ReportSpam  = "spam"
	ReportPorn  = "porn"
	ReportScam  = "scam"
	ReportAbuse = "abuse"
	ReportOther = "other"

	ReportPending   int16 = 0
	ReportHandled   int16 = 1
	ReportDismissed int16 = 2

	// PlatformOfficialAddr 运营台官方发帖使用的内部地址，不是钱包。
	PlatformOfficialAddr = "square-official"
	PlatformOfficialName = "谈信广场"

	GateOpen    = "open"
	GateClosed  = "closed"
	GateLimited = "limited"

	ModerationDefault = "default"
	ModerationStrict  = "strict"

	DefaultGateMessage = "广场功能已经关闭"
)

// OfficialPostTags 返回平台官方帖允许的标签。
func OfficialPostTags() []string {
	return []string{OfficialPostTagAnnouncement, OfficialPostTagMessage, OfficialPostTagActivity}
}

type Post struct {
	ID              int64
	AuthorAddr      string
	AuthorName      string
	AuthorKind      int16
	AuthorAvatar    string
	AuthorIntro     string
	Title           string
	Intro           string
	Content         string
	Tag             string // 官方帖标签：公告/消息/活动；个人帖为空
	MediaJSON       []byte
	CommentsEnabled int16
	Status          int16
	CommentCount    int64
	LikeCount       int64
	ViewCount       int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
	PinnedAt        *time.Time
	PublishAt       *time.Time
	ExpireAt        *time.Time
	ExpireAfterSec  int64
	Liked           bool
	AuthorGold      bool
}

func (p *Post) Pinned() bool {
	return p != nil && p.PinnedAt != nil
}

func (p *Post) ContentFormat() string {
	if p != nil && p.AuthorAddr == PlatformOfficialAddr {
		return "markdown"
	}
	return "plain"
}

type Comment struct {
	ID          int64
	PostID      int64
	AuthorAddr  string
	AuthorName  string
	Content     string
	ParentID    int64
	ReplyToID   int64
	ReplyToAddr string
	ReplyToName string
	ReplyCount  int64
	Status      int16
	Floor       int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
	// Deleted 一级评论已不可见但仍有可见回复时，列表用占位楼。
	Deleted bool
	// Replies 非 nil 时表示楼中楼预览，空切片也要返回给客户端。
	Replies []Comment
}

// OfficialPostReply 运营台看到的一篇平台官方帖的用户回复汇总。
// Title、Intro、Content 只用于生成 PostTitle，不返回给前端。
type OfficialPostReply struct {
	PostID           int64
	Title            string
	Intro            string
	Content          string
	PostTitle        string
	PostStatus       int16
	ReplyCount       int64
	LatestCommentID  int64
	LatestAuthorName string
	LatestAuthorAddr string
	LatestContent    string
	LatestAt         time.Time
}

// ReplyNotice 广场内评论/回复收件箱。PostStatus、CommentStatus 为列表时 JOIN 出来的当前状态。
type ReplyNotice struct {
	ID            int64
	Recipient     string
	ActorAddr     string
	ActorName     string
	PostID        int64
	PostTitle     string
	RootID        int64
	CommentID     int64
	ReplyToID     int64
	Content       string
	ReadAt        *time.Time
	CreatedAt     time.Time
	PostStatus    int16
	CommentStatus int16
}

// OfficialName 平台官方号可切换的展示名。帖子仍挂在 square-official 上，用 author_name 区分。
type OfficialName struct {
	ID         int64
	Name       string
	Avatar     string
	Intro      string
	DraftCount int64
	PostCount  int64
	CreatedAt  time.Time
}

type OfficialAccount struct {
	Address       string
	Name          string
	Avatar        string
	Intro         string
	Status        int16
	ApplyID       int64
	BannedAt      *time.Time
	BanReason     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Gold          bool
	GoldAt        *time.Time
	PostCount     int64
	FollowerCount int64
	ReportCount   int64
	Followed      bool
}

type FollowProfile struct {
	Addr       string
	Kind       int16
	Name       string
	Avatar     string
	FollowedAt time.Time
}

type Report struct {
	ID           int64
	ReporterAddr string
	TargetType   string
	TargetID     int64
	Reason       string
	Detail       string
	Status       int16
	HandledBy    string
	HandledAt    *time.Time
	CreatedAt    time.Time
	PostTitle    string
	PostIntro    string
	AuthorAddr   string
	AuthorName   string
	AuthorKind   int16
	PostStatus   int16
}

type AccountStat struct {
	Address           string
	AuthorName        string
	IsOfficial        bool
	OfficialStatus    int16
	PostCount         int64
	FollowerCount     int64
	FollowingCount    int64
	Avatar            string
	Intro             string
	Gold              bool
	OfficialBanReason string
	ReportCount       int64
	UserBanned        bool
	UserBanReason     string
	JoinedAt          *time.Time
}

type AuthorRank struct {
	Address      string
	AuthorName   string
	AuthorKind   int16
	AuthorAvatar string
	IsOfficial   bool
	PostCount    int64
	LikeCount    int64
	ViewCount    int64
	CommentCount int64
}

type AuthorTraffic struct {
	TotalPosts    int64
	TotalViews    int64
	TotalLikes    int64
	TotalComments int64
}

type UserBan struct {
	Address    string
	Reason     string
	Operator   string
	AuthorName string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type OfficialApply struct {
	ID           int64
	Address      string
	Name         string
	Avatar       string
	Intro        string
	Status       string
	Reviewer     string
	ReviewRemark string
	CreatedAt    time.Time
	ReviewedAt   *time.Time
}

type Admin struct {
	ID           int64
	Username     string
	PasswordHash string
	Role         string
	Status       int16
	CreatedAt    time.Time
	// Permissions 为已落库的权限码。PermReady 为 false 表示旧数据尚未回填。
	Permissions []string
	PermReady   bool
}

type Moderation struct {
	ID           int64
	TargetType   string
	TargetID     int64
	Verdict      string
	Categories   []byte
	Reason       string
	Model        string
	LatencyMS    int
	OverriddenBy *string
	OverriddenAt *time.Time
	FinalVerdict *string
	CreatedAt    time.Time
}

type Audit struct {
	ID         int64
	AdminID    int64
	Username   string
	Action     string
	TargetType string
	TargetID   string
	Extra      string
	ClientIP   string
	CreatedAt  time.Time
}

type StatCounts struct {
	Posts    int64
	Comments int64
	Views    int64
	Likes    int64
}

type StatDay struct {
	Date     string
	Posts    int64
	Comments int64
	Views    int64
	Likes    int64
}

type OverviewStats struct {
	Timezone     string
	Days         int
	Today        StatCounts
	Yesterday    StatCounts
	Total        StatCounts
	Series       []StatDay
	AuthorAddrs  []string
	AuthorTotal  int64
	AuthorSeries []StatDay
}

type PageQuery struct {
	Page     int
	PageSize int
}

func (p PageQuery) Normalize() PageQuery {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize < 1 {
		p.PageSize = 20
	}
	if p.PageSize > 100 {
		p.PageSize = 100
	}
	return p
}

type SquareSettings struct {
	GateMode        string
	GateMessage     string
	AllowOfficials  bool
	ModerationLevel string
	Posters         []PosterAllow
	UpdatedAt       time.Time
}

func DefaultSquareSettings() SquareSettings {
	return SquareSettings{
		GateMode:        GateOpen,
		GateMessage:     DefaultGateMessage,
		AllowOfficials:  true,
		ModerationLevel: ModerationDefault,
		Posters:         []PosterAllow{},
	}
}

func (s SquareSettings) ClosedMessage() string {
	msg := strings.TrimSpace(s.GateMessage)
	if msg == "" {
		return DefaultGateMessage
	}
	return msg
}

func (s SquareSettings) CanRead() bool {
	return s.GateMode != GateClosed
}

type PosterAllow struct {
	Address   string
	Remark    string
	Operator  string
	CreatedAt time.Time
}

func (p PageQuery) Offset() int {
	return (p.Page - 1) * p.PageSize
}
