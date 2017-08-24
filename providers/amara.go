package providers

import (
	"errors"
	"net/url"
	"strconv"

	"github.com/NYTimes/amara"
	"github.com/NYTimes/gizmo/config"
	captionsConfig "github.com/NYTimes/video-captions-api/config"
	"github.com/NYTimes/video-captions-api/database"
	log "github.com/Sirupsen/logrus"
)

// AmaraProvider amara client wrapper that implements the Provider interface
type AmaraProvider struct {
	*amara.Client
	logger *log.Logger
}

// AmaraConfig holds Amara related config
type AmaraConfig struct {
	Username string `envconfig:"AMARA_USERNAME"`
	Team     string `envconfig:"AMARA_TEAM"`
	Token    string `envconfig:"AMARA_TOKEN"`
}

// NewAmaraProvider creates an AmaraProvider
func NewAmaraProvider(cfg *AmaraConfig, svcCfg *captionsConfig.CaptionsServiceConfig) Provider {
	return &AmaraProvider{
		amara.NewClient(cfg.Username, cfg.Token, cfg.Team),
		svcCfg.Logger,
	}
}

// LoadAmaraConfigFromEnv loads Amara username, token and team from environment
func LoadAmaraConfigFromEnv() AmaraConfig {
	var providerConfig AmaraConfig
	config.LoadEnvConfig(&providerConfig)
	return providerConfig
}

// GetName returns provider name
func (c *AmaraProvider) GetName() string {
	return "amara"
}

// Download download latest subtitle version from Amara
func (c *AmaraProvider) Download(id, _ string) ([]byte, error) {
	sub, err := c.GetSubtitles(id, "en")
	if err != nil {
		return nil, err
	}
	return []byte(sub.Subtitles), nil
}

// GetProviderJob returns current job status from Amara
func (c *AmaraProvider) GetProviderJob(id string) (*database.ProviderJob, error) {
	subs, err := c.GetSubtitles(id, "en")
	status := "in review"
	if err != nil {
		return nil, err
	}
	lang, err := c.GetLanguage(id, "en")
	if err != nil {
		return nil, err
	}

	if lang.SubtitlesComplete {
		status = "delivered"
	}

	return &database.ProviderJob{
		ID:      id,
		Status:  status,
		Details: "Version " + strconv.Itoa(subs.VersionNumber),
		Params: map[string]string{
			"SubVersion": strconv.Itoa(subs.VersionNumber),
		},
	}, nil
}

// DispatchJob creates a video and adds subtitle to it
func (c *AmaraProvider) DispatchJob(job *database.Job) error {
	params := url.Values{}

	for k, v := range job.ProviderParams {
		params.Add(k, v)
	}

	params.Add("team", c.Team)
	params.Add("video_url", job.MediaURL)

	video, err := c.CreateVideo(params)
	if err != nil || video.ID == "" {
		return errors.New("failed to create video in amara")
	}
	subs, err := c.CreateSubtitles(video.ID, job.Language, "vtt", params)
	if err != nil {
		return err
	}

	lang, err := c.UpdateLanguage(video.ID, job.Language, false)
	if err != nil {
		return err
	}

	job.ProviderParams["ProviderID"] = video.ID
	job.ProviderParams["SubVersion"] = strconv.Itoa(subs.VersionNumber)
	return nil
}
