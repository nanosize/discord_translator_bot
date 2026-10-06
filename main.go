package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"gopkg.in/yaml.v3"
)

type channelMapping struct {
	Source uint64 `yaml:"source_channel_id"`
	Target uint64 `yaml:"target_channel_id"`
}

type botConfig struct {
	ChannelMappings    []channelMapping  `yaml:"channel_mappings"`
	ExcludedChannelIDs []uint64          `yaml:"excluded_channel_ids"`
	FlagMap            map[string]string `yaml:"flag_map"`
	MaxPerMinute       int               `yaml:"max_per_minute"`
}

type rateLimiter struct {
	mu        sync.Mutex
	events    []time.Time
	processed map[string]time.Time
	inFlight  map[string]struct{}
	limit     int
}

type application struct {
	config     botConfig
	translator *Translator
	limiter    *rateLimiter
	channelMu  sync.Mutex
	channels   map[string]*discordgo.Channel
}

func loadDotEnv() error {
	path := os.Getenv("ENV_FILE")
	if path == "" {
		path = ".env"
	}
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
			if decoded, decodeErr := strconv.Unquote(value); decodeErr == nil {
				value = decoded
			}
		} else if len(value) >= 2 && strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") {
			value = value[1 : len(value)-1]
		} else if comment := strings.Index(value, " #"); comment >= 0 {
			value = strings.TrimSpace(value[:comment])
		}
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func loadConfig() (botConfig, error) {
	path := os.Getenv("CONFIG_FILE")
	if path == "" {
		path = "config.yml"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return botConfig{}, fmt.Errorf("read config file: %w", err)
	}
	var config botConfig
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&config); err != nil {
		return botConfig{}, fmt.Errorf("parse config file: %w", err)
	}
	if config.MaxPerMinute <= 0 {
		config.MaxPerMinute = 30
	}
	if config.FlagMap == nil {
		config.FlagMap = map[string]string{}
	}
	for i, mapping := range config.ChannelMappings {
		if mapping.Source == 0 || mapping.Target == 0 {
			return botConfig{}, fmt.Errorf("channel_mappings[%d] must have non-zero source_channel_id and target_channel_id", i)
		}
		if mapping.Source == mapping.Target {
			return botConfig{}, fmt.Errorf("channel_mappings[%d] source and target channel IDs must differ", i)
		}
	}
	for i, id := range config.ExcludedChannelIDs {
		if id == 0 {
			return botConfig{}, fmt.Errorf("excluded_channel_ids[%d] must be a non-zero channel ID", i)
		}
	}
	if value := os.Getenv("MAX_PER_MINUTE"); value != "" {
		limit, err := strconv.Atoi(value)
		if err != nil || limit <= 0 {
			return botConfig{}, fmt.Errorf("MAX_PER_MINUTE must be a positive integer")
		}
		config.MaxPerMinute = limit
	}
	return config, nil
}

func newRateLimiter(limit int) *rateLimiter {
	return &rateLimiter{limit: limit, processed: make(map[string]time.Time), inFlight: make(map[string]struct{})}
}

func (l *rateLimiter) allow(messageID, language string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-time.Minute)
	first := 0
	for first < len(l.events) && l.events[first].Before(cutoff) {
		first++
	}
	l.events = append([]time.Time(nil), l.events[first:]...)
	for key, created := range l.processed {
		if created.Before(now.Add(-24 * time.Hour)) {
			delete(l.processed, key)
		}
	}
	key := messageID + ":" + language
	_, active := l.inFlight[key]
	if _, exists := l.processed[key]; exists || active || len(l.events) >= l.limit {
		return false
	}
	l.inFlight[key] = struct{}{}
	l.events = append(l.events, now)
	return true
}

func (l *rateLimiter) finish(messageID, language string, succeeded bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	key := messageID + ":" + language
	delete(l.inFlight, key)
	if succeeded {
		l.processed[key] = time.Now()
	}
}

func (a *application) getChannel(s *discordgo.Session, channelID string) (*discordgo.Channel, error) {
	if channel, err := s.State.Channel(channelID); err == nil {
		return channel, nil
	}
	a.channelMu.Lock()
	channel := a.channels[channelID]
	a.channelMu.Unlock()
	if channel != nil {
		return channel, nil
	}
	channel, err := s.Channel(channelID)
	if err != nil {
		return nil, err
	}
	a.channelMu.Lock()
	a.channels[channelID] = channel
	a.channelMu.Unlock()
	return channel, nil
}

func (a *application) isExcluded(s *discordgo.Session, channelID string) bool {
	if channelID == "" {
		return false
	}
	ids := map[string]struct{}{channelID: {}}
	channel, err := a.getChannel(s, channelID)
	if err == nil && channel != nil && channel.IsThread() && channel.ParentID != "" {
		ids[channel.ParentID] = struct{}{}
	}
	for _, id := range a.config.ExcludedChannelIDs {
		if _, ok := ids[strconv.FormatUint(id, 10)]; ok {
			return true
		}
	}
	return false
}

func (a *application) sourceChannelID(s *discordgo.Session, channelID string) string {
	channel, err := a.getChannel(s, channelID)
	if err == nil && channel != nil && channel.IsThread() && channel.ParentID != "" {
		return channel.ParentID
	}
	return channelID
}

func (a *application) onReady(s *discordgo.Session, _ *discordgo.Ready) {
	commands := []*discordgo.ApplicationCommand{
		{
			Name: "translate", Description: "テキスト翻訳",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "text", Description: "本文", Required: true, MaxLength: 4000},
				{Type: discordgo.ApplicationCommandOptionString, Name: "target", Description: "翻訳先の言語コード (例: JA)", Required: true, MaxLength: 16},
			},
		},
		{
			Name: "detect", Description: "言語検出",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "text", Description: "本文", Required: true, MaxLength: 4000},
			},
		},
	}
	if _, err := s.ApplicationCommandBulkOverwrite(s.State.User.ID, "", commands); err != nil {
		log.Printf("Could not sync slash commands: %v", err)
		return
	}
	log.Printf("Logged in as %s", s.State.User.Username)
}

func (a *application) onMessage(s *discordgo.Session, event *discordgo.MessageCreate) {
	message := event.Message
	if message == nil || message.Author == nil || message.Author.Bot {
		return
	}
	if a.isExcluded(s, message.ChannelID) {
		return
	}
	parentChannelID := a.sourceChannelID(s, message.ChannelID)
	for _, mapping := range a.config.ChannelMappings {
		if parentChannelID != strconv.FormatUint(mapping.Source, 10) {
			continue
		}
		if strings.TrimSpace(message.Content) == "" || !a.limiter.allow(message.ID, "EN") {
			return
		}
		succeeded := false
		defer func() { a.limiter.finish(message.ID, "EN", succeeded) }()
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		result, err := a.translator.Translate(ctx, message.Content, "EN")
		if err != nil {
			log.Printf("Channel translation failed: %v", err)
			return
		}
		name := message.Author.Username
		if message.Member != nil && message.Member.Nick != "" {
			name = message.Member.Nick
		}
		content := fmt.Sprintf("**%s**:\n%s", name, limitDiscordText(result.Text, 1900))
		targetID := strconv.FormatUint(mapping.Target, 10)
		if a.isExcluded(s, targetID) {
			return
		}
		targetChannel, channelErr := a.getChannel(s, targetID)
		if channelErr != nil {
			log.Printf("Could not inspect target channel %s: %v", targetID, channelErr)
			return
		}
		messageSend := &discordgo.MessageSend{
			Content:         content,
			AllowedMentions: &discordgo.MessageAllowedMentions{},
		}
		if targetChannel.Type == discordgo.ChannelTypeGuildForum {
			name := limitDiscordText(strings.TrimSpace(fmt.Sprintf("%s: %s", name, message.Content)), 100)
			_, err = s.ForumThreadStartComplex(targetID, &discordgo.ThreadStart{Name: name, AutoArchiveDuration: 1440}, messageSend)
		} else {
			_, err = s.ChannelMessageSendComplex(targetID, messageSend)
		}
		if err != nil {
			log.Printf("Could not forward translated message: %v", err)
			return
		}
		succeeded = true
		return
	}
}

func (a *application) onReaction(s *discordgo.Session, event *discordgo.MessageReactionAdd) {
	if event == nil || (s.State.User != nil && event.UserID == s.State.User.ID) {
		return
	}
	if a.isExcluded(s, event.ChannelID) {
		return
	}
	emoji := event.Emoji.Name
	target := a.config.FlagMap[emoji]
	if target == "" || !everyoneCanSend(s, event.ChannelID, event.GuildID) {
		return
	}
	if !a.limiter.allow(event.MessageID, target) {
		return
	}
	succeeded := false
	defer func() { a.limiter.finish(event.MessageID, target, succeeded) }()
	message, err := s.ChannelMessage(event.ChannelID, event.MessageID)
	if err != nil {
		log.Printf("Could not fetch reacted message: %v", err)
		return
	}
	if strings.TrimSpace(message.Content) == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	result, err := a.translator.Translate(ctx, message.Content, target)
	if err != nil {
		log.Printf("Reaction translation failed: %v", err)
		return
	}
	content := fmt.Sprintf("%s **%s**:\n%s", emoji, target, limitDiscordText(result.Text, 1800))
	_, err = s.ChannelMessageSendComplex(event.ChannelID, &discordgo.MessageSend{
		Content: content,
		Reference: &discordgo.MessageReference{
			MessageID: message.ID, ChannelID: message.ChannelID, GuildID: message.GuildID,
		},
		AllowedMentions: &discordgo.MessageAllowedMentions{},
	})
	if err != nil {
		log.Printf("Could not send reaction translation: %v", err)
		return
	}
	succeeded = true
}

func everyoneCanSend(s *discordgo.Session, channelID, guildID string) bool {
	if guildID == "" {
		return false
	}
	channel, err := s.Channel(channelID)
	if err != nil {
		return false
	}
	guild, err := s.Guild(guildID)
	if err != nil {
		return false
	}
	var permissions int64
	for _, role := range guild.Roles {
		if role.ID == guildID {
			permissions = role.Permissions
			break
		}
	}
	for _, overwrite := range channel.PermissionOverwrites {
		if overwrite.Type == discordgo.PermissionOverwriteTypeRole && overwrite.ID == guildID {
			permissions &^= overwrite.Deny
			permissions |= overwrite.Allow
			break
		}
	}
	return permissions&(discordgo.PermissionSendMessages|discordgo.PermissionSendMessagesInThreads) != 0
}

func (a *application) onInteraction(s *discordgo.Session, event *discordgo.InteractionCreate) {
	if event == nil || event.ApplicationCommandData().Name == "" {
		return
	}
	if a.isExcluded(s, event.ChannelID) {
		_ = s.InteractionRespond(event.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: "このチャンネルでは翻訳が無効です。", Flags: discordgo.MessageFlagsEphemeral},
		})
		return
	}
	data := event.ApplicationCommandData()
	var text, target string
	for _, option := range data.Options {
		switch option.Name {
		case "text":
			text = option.StringValue()
		case "target":
			target = option.StringValue()
		}
	}
	if err := s.InteractionRespond(event.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	}); err != nil {
		log.Printf("Could not acknowledge slash command: %v", err)
		return
	}

	var content string
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if data.Name == "translate" {
		result, err := a.translator.Translate(ctx, text, target)
		if err != nil {
			content = "翻訳できませんでした。設定とプロバイダーの応答を確認してください。"
			log.Printf("Slash translation failed: %v", err)
		} else {
			content = fmt.Sprintf("**→%s**\n%s", strings.ToUpper(target), limitDiscordText(result.Text, 1800))
		}
	} else if data.Name == "detect" {
		language, err := a.translator.Detect(ctx, text)
		if err != nil {
			content = "言語を判定できませんでした。設定とプロバイダーの応答を確認してください。"
			log.Printf("Language detection failed: %v", err)
		} else {
			content = "Detect: **" + language + "**"
		}
	} else {
		content = "未対応のコマンドです。"
	}
	if _, err := s.InteractionResponseEdit(event.Interaction, &discordgo.WebhookEdit{Content: &content}); err != nil {
		log.Printf("Could not send slash command response: %v", err)
	}
}

func limitDiscordText(text string, maxRunes int) string {
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes-1]) + "…"
}

func main() {
	if err := loadDotEnv(); err != nil {
		log.Fatalf("Could not load environment: %v", err)
	}
	token := os.Getenv("DISCORD_TOKEN")
	if token == "" {
		log.Fatal("DISCORD_TOKEN is required")
	}
	config, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}
	app := &application{
		config: config, translator: newTranslator(), limiter: newRateLimiter(config.MaxPerMinute), channels: make(map[string]*discordgo.Channel),
	}
	if !app.translator.configured() {
		log.Fatalf("translation provider %q is unsupported or missing required settings; check TRANSLATION_PROVIDER and the provider variables in .env", app.translator.Provider)
	}
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatalf("Could not create Discord session: %v", err)
	}
	session.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages | discordgo.IntentsGuildMessageReactions | discordgo.IntentsMessageContent
	session.AddHandler(app.onReady)
	session.AddHandler(app.onMessage)
	session.AddHandler(app.onReaction)
	session.AddHandler(app.onInteraction)
	if err := session.Open(); err != nil {
		log.Fatalf("Could not connect to Discord: %v", err)
	}
	defer session.Close()
	log.Printf("Discord translation bot is running with provider %q", app.translator.Provider)
	select {}
}
