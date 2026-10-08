# Discord Translator Bot

<p align="center">
  <img src="https://raw.githubusercontent.com/nanosize/discord_translator_bot/main/assets/icon.png" alt="Discord Translator Bot icon" width="160">
</p>

A Discord translation bot written in Go. It supports automatic translation between channels, flag-reaction translations, and the `/translate` and `/detect` commands. Choose one translation provider: Google Cloud Translation, DeepL, Azure Translator, Gemini, OpenAI, or an OpenAI-compatible API.

## Quick start with Docker Compose

### Requirements

- Docker Engine with the Docker Compose plugin
- A Discord bot token with **Message Content Intent** enabled
- An API key for one supported translation provider

### 1. Download the project and create the configuration files

```sh
git clone https://github.com/nanosize/discord_translator_bot.git
cd discord_translator_bot
cp .env.example .env
cp config.example.yml config.yml
```

### 2. Set your token and translation provider

Open `.env` and set `DISCORD_TOKEN`. `TRANSLATION_PROVIDER` defaults to `google`; add the API key required for the provider you want to use. The provider table below lists each required variable.

For example, with Google Cloud Translation:

```dotenv
DISCORD_TOKEN=your-discord-bot-token
TRANSLATION_PROVIDER=google
GOOGLE_TRANSLATE_API_KEY=your-google-translate-api-key
```

Do not commit `.env` or share it publicly. It contains your bot token and provider credentials.

### 3. Set channel IDs

Open `config.yml` and add the channel mappings for your Discord server. To copy a channel ID, enable **Developer Mode** in Discord, right-click the channel, and select **Copy Channel ID**.

```yaml
channel_mappings:
  - source_channel_id: 123456789012345678
    target_channel_id: 234567890123456789
```

Replace the example IDs with your own channel IDs. The bot translates mapped messages into English. See [`config.example.yml`](config.example.yml) for the complete configuration format.

### 4. Start the bot

```sh
docker compose pull
docker compose up -d
```

Compose creates a persistent `bot-config` volume and initializes it from the example configuration included in the image. Copy your edited `config.yml` into that volume, then restart the bot to load it:

```sh
docker compose cp ./config.yml discord-translator-bot:/data/config.yml
docker compose restart discord-translator-bot
```

Follow the startup log and confirm the bot connected successfully:

```sh
docker compose logs -f discord-translator-bot
```

The bot needs permission to view the configured channels, read message history, and send messages. It also needs permission to create posts if a destination is a forum channel.

## Translation providers

Set `TRANSLATION_PROVIDER` to one provider and configure the required variables in `.env`.

| `TRANSLATION_PROVIDER` | Required environment variables |
|---|---|
| `google` | `GOOGLE_TRANSLATE_API_KEY` |
| `deepl` | `DEEPL_AUTH_KEY` |
| `azure` | `AZURE_TRANSLATOR_KEY`; some resources also require `AZURE_TRANSLATOR_REGION` |
| `gemini` | `GEMINI_API_KEY`, `GEMINI_MODEL` |
| `openai` | `OPENAI_API_KEY`, `OPENAI_MODEL` |
| `openai-compatible` | `OPENAI_COMPATIBLE_API_KEY`, `OPENAI_COMPATIBLE_MODEL`, `OPENAI_COMPATIBLE_BASE_URL` |

DeepL uses its Free API by default. Set `DEEPL_API_URL` to use the Pro API. See `.env.example` for custom URL settings for Google, Azure, OpenAI, and Gemini. Variables already set in the shell or service take precedence over values in `.env`.

## Configuration options

- `channel_mappings`: Automatically translates messages from `source_channel_id` to `target_channel_id`. The target language is English.
- `excluded_channel_ids`: Channels the bot will not translate. If you specify a forum channel, its post threads are also excluded and slash commands are disabled there.
- `flag_map`: Maps flag emoji to target language codes. Emoji are matched exactly.
- `max_per_minute`: Combined rate limit for automatic and reaction translations. Override it with the `MAX_PER_MINUTE` environment variable.

Server administrators can use `/reaction-translate` to enable or disable reaction translations for a channel and check its status. Examples: `/reaction-translate disable channel:#chat`, `/reaction-translate enable channel:#general`, and `/reaction-translate status channel:#general`. The setting is saved in `reaction_translation_disabled_channel_ids`. The bot must be able to write to `config.yml` for this command to save changes. Reaction translations are always disabled in channels listed in `excluded_channel_ids`.

You can use a forum as the source in a channel mapping. Messages in forum posts are translated. If the destination is a regular channel, translations are sent there. If the destination is a forum, the bot creates a translated post for each message. The bot needs permission to create posts in the destination forum.

## Docker Hub image

The public image is [`nanosize23/discord-translator-bot`](https://hub.docker.com/r/nanosize23/discord-translator-bot). `compose.yaml` uses this image. The image does not contain your `.env` file or API keys.

### Updating the image

```sh
docker compose pull
docker compose up -d
```

The `bot-config` volume keeps your configuration when the container is updated. To change `config.yml`, copy the edited file into the volume and restart the service:

```sh
docker compose cp ./config.yml discord-translator-bot:/data/config.yml
docker compose restart discord-translator-bot
```

### Portainer

1. Open **Stacks → Add stack → Repository**, then enter this GitHub repository and the `main` branch.
2. Set the Compose path to `compose.yaml`.
3. Add `DISCORD_TOKEN`, `TRANSLATION_PROVIDER`, and the API key for your chosen provider under **Environment variables**, then deploy the stack.

Portainer does not load this repository's `.env` file automatically, so add the values as stack environment variables. To use a custom `config.yml`, copy it into the `bot-config` volume before restarting the stack. A new volume is initialized from `config.example.yml` in the image.

## Run from source

1. Copy `.env.example` to `.env` and set `DISCORD_TOKEN` and the credentials for your chosen translation provider.
2. Copy `config.example.yml` to `config.yml` and set your channel IDs.
3. Build and run the bot:

   ```sh
   go build -o discord-translator-bot .
   ./discord-translator-bot
   ```

Set `ENV_FILE` and `CONFIG_FILE` to use different files. The default value of `TRANSLATION_PROVIDER` is `google`. The bot exits with an error at startup if the provider is unsupported or a required setting is missing.

## systemd

The service file assumes the application is installed at `/opt/discord_translator_bot`. Create `.env` and `config.yml` in that directory, then build the binary and enable the service. The `discordbot` user must be able to read the binary and configuration files and access the working directory.

## License

This project is licensed under the [GNU General Public License v3.0 only](LICENSE).
