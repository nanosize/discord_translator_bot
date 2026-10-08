# Discord Translator Bot

<p align="center">
  <img src="https://raw.githubusercontent.com/nanosize/discord_translator_bot/main/assets/icon.png" alt="Discord Translator Bot icon" width="160">
</p>

A Discord translation bot written in Go. It supports automatic translation between channels, flag-reaction translations, and the `/translate` and `/detect` commands. Choose one translation provider: Google Cloud Translation, DeepL, Azure Translator, Gemini, OpenAI, or an OpenAI-compatible API.

## Setup

1. Copy `.env.example` to `.env`, then set your Discord bot token and credentials for the translation provider you chose.
2. Copy `config.example.yml` to `config.yml`.
3. Build and run the bot:

   ```sh
   go build -o discord-translator-bot .
   ./discord-translator-bot
   ```

Variables from `.env` have lower precedence than variables provided by the shell or service, so they do not overwrite existing environment variables. Set `ENV_FILE` and `CONFIG_FILE` to use different files. The default value of `TRANSLATION_PROVIDER` is `google`. The bot exits with an error at startup if the provider is unsupported or a required setting is missing.

### Translation providers

`.env.example` lists the environment variables and example values for every provider. Configure only the section for the provider you use.

| `TRANSLATION_PROVIDER` | Required environment variables |
|---|---|
| `google` | `GOOGLE_TRANSLATE_API_KEY` |
| `deepl` | `DEEPL_AUTH_KEY` |
| `azure` | `AZURE_TRANSLATOR_KEY`; some resources also require `AZURE_TRANSLATOR_REGION` |
| `gemini` | `GEMINI_API_KEY`, `GEMINI_MODEL` |
| `openai` | `OPENAI_API_KEY`, `OPENAI_MODEL` |
| `openai-compatible` | `OPENAI_COMPATIBLE_API_KEY`, `OPENAI_COMPATIBLE_MODEL`, `OPENAI_COMPATIBLE_BASE_URL` |

DeepL uses its Free API by default. Set `DEEPL_API_URL` to use the Pro API. See `.env.example` for custom URL settings for Google, Azure, OpenAI, and Gemini.

## `config.yml`

Copy `config.example.yml` and set channel IDs for your server. To copy an ID, enable Developer Mode in Discord, right-click a channel, and select **Copy Channel ID**.

- `channel_mappings`: Automatically translates messages from `source_channel_id` to `target_channel_id`. The target language is English.
- `excluded_channel_ids`: Channels the bot will not translate. If you specify a forum channel, its post threads are also excluded and slash commands are disabled there.
- `flag_map`: Maps flag emoji to target language codes. Emoji are matched exactly.
- `max_per_minute`: Combined rate limit for automatic and reaction translations. Override it with the `MAX_PER_MINUTE` environment variable.

Server administrators can use `/reaction-translate` to enable or disable reaction translations for a channel and check its status. Examples: `/reaction-translate disable channel:#chat`, `/reaction-translate enable channel:#general`, and `/reaction-translate status channel:#general`. The setting is saved in `reaction_translation_disabled_channel_ids`. The bot must be able to write to `config.yml` for this command to save changes. Reaction translations are always disabled in channels listed in `excluded_channel_ids`.

You can use a forum as the source in a channel mapping. Messages in forum posts are translated. If the destination is a regular channel, translations are sent there. If the destination is a forum, the bot creates a translated post for each message. The bot needs permission to create posts in the destination forum.

## Docker

The public image is [`nanosize23/discord-translator-bot`](https://hub.docker.com/r/nanosize23/discord-translator-bot). `compose.yaml` uses this image. Your `.env` file and API keys are not included in the image.

### Docker Compose

```sh
cp .env.example .env
# Set DISCORD_TOKEN and the key for your chosen provider in .env.
docker compose pull
docker compose up -d
docker compose logs -f
```

### Portainer

1. Open **Stacks → Add stack → Repository**, then enter this GitHub repository and the `main` branch.
2. Set the Compose path to `compose.yaml`.
3. Add `DISCORD_TOKEN`, `TRANSLATION_PROVIDER`, and the API key for your chosen provider under **Environment variables**, then deploy the stack.

See the provider table above for the required variables. Portainer does not load this repository's `.env` file automatically, so add the values as stack environment variables. The named `bot-config` volume stores the configuration and keeps it when the container is updated.

To apply a new image in Portainer, select **Pull and redeploy**. With Docker Compose, run `docker compose pull && docker compose up -d`.

A new `bot-config` volume is initialized from `config.example.yml` in the image. If you want to use an existing `config.yml`, copy it into the volume before the first start. Do not bake `.env` files or real configuration files into the image.

Enable **Message Content Intent** in the Discord Developer Portal. The bot needs permission to view the target channels, read message history, and send messages. It also needs permission to create posts when translating to a forum.

## systemd

The service file assumes the application is installed at `/opt/discord_translator_bot`. Create `.env` and `config.yml` in that directory, then build the binary and enable the service. The `discordbot` user must be able to read the binary and configuration files and access the working directory.

## License

This project is licensed under the [GNU General Public License v3.0 only](LICENSE).
