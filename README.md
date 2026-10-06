# Discord 翻訳 Bot

Go 製の Discord 翻訳 Bot です。チャンネル間の自動翻訳、国旗リアクション翻訳、`/translate`、`/detect` に対応しています。翻訳先は Google Cloud Translation、DeepL、Azure Translator、Gemini、OpenAI、OpenAI 互換APIから1つ選びます。

## セットアップ

1. `.env.example` を `.env` にコピーし、Discord Bot トークンと選んだ翻訳プロバイダの認証情報を設定します。
2. `config.example.yml` を `config.yml` にコピーします。
3. `go build -o discord-translator-bot .` を実行し、`./discord-translator-bot` で起動します。

`.env` の環境変数はシェルやサービスから渡された環境変数より優先度が低く、既存の環境変数を上書きしません。`ENV_FILE` と `CONFIG_FILE` で読み込むファイルを変更できます。`TRANSLATION_PROVIDER` の既定値は `google` です。未対応のプロバイダや必須設定の不足があると起動時にエラーになります。

### プロバイダの設定

`.env.example` に全プロバイダの変数名と記入例があります。利用するプロバイダの欄だけを設定してください。

| `TRANSLATION_PROVIDER` | 必須の環境変数 |
|---|---|
| `google` | `GOOGLE_TRANSLATE_API_KEY` |
| `deepl` | `DEEPL_AUTH_KEY` |
| `azure` | `AZURE_TRANSLATOR_KEY`。リソースによって `AZURE_TRANSLATOR_REGION` も必要です |
| `gemini` | `GEMINI_API_KEY`, `GEMINI_MODEL` |
| `openai` | `OPENAI_API_KEY`, `OPENAI_MODEL` |
| `openai-compatible` | `OPENAI_COMPATIBLE_API_KEY`, `OPENAI_COMPATIBLE_MODEL`, `OPENAI_COMPATIBLE_BASE_URL` |

DeepL は既定で Free API を使います。Pro APIの場合は `DEEPL_API_URL` を設定してください。Google、Azure、OpenAI、GeminiのカスタムURLも `.env.example` を参照してください。

## `config.yml`

`config.example.yml` をコピーして、サーバーに合わせたチャンネルIDを設定します。IDはDiscordの開発者モードを有効にしてチャンネルを右クリックし、「チャンネルIDをコピー」で取得できます。

- `channel_mappings`: `source_channel_id` から `target_channel_id` へ自動翻訳します。翻訳先は英語です。
- `excluded_channel_ids`: Botが翻訳しないチャンネルです。ここにフォーラムIDを指定すると、そのフォーラム内の投稿スレッドも対象外になります。スラッシュコマンドも無効になります。
- `flag_map`: 国旗絵文字から翻訳先言語コードへの対応です。絵文字は完全一致で判定します。
- `max_per_minute`: 自動翻訳とリアクション翻訳の合計上限です。`MAX_PER_MINUTE` 環境変数で上書きできます。

サーバー管理権限を持つ人は `/reaction-translate` でチャンネルごとのリアクション翻訳を切り替えたり、状態を確認できます。例: `/reaction-translate disable channel:#雑談`、`/reaction-translate enable channel:#general`、`/reaction-translate status channel:#general`。設定は `reaction_translation_disabled_channel_ids` に保存されます。このコマンドで設定する場合、Botが `config.yml` を書き込める必要があります。`excluded_channel_ids` に登録されたチャンネルでは、リアクション翻訳も常に無効です。

マッピングの送信元にフォーラムを指定すると、フォーラム投稿内のメッセージを翻訳します。送信先が通常のチャンネルならそこへ転送し、送信先がフォーラムならメッセージごとに翻訳投稿を作成します。フォーラム投稿の作成には、Botに投稿先フォーラムで投稿権限が必要です。

## Docker

公開イメージは [`nanosize23/discord-translator-bot`](https://hub.docker.com/r/nanosize23/discord-translator-bot) です。`compose.yaml` はこのイメージを使います。`.env` やAPIキーはイメージに含まれません。

### Docker Compose

```sh
cp .env.example .env
# .env に DISCORD_TOKEN と選択したプロバイダのキーを設定
docker compose pull
docker compose up -d
docker compose logs -f
```

### Portainer

1. **Stacks → Add stack → Repository** を開き、このGitHubリポジトリとブランチ `main` を指定します。
2. Compose pathに `compose.yaml` を入力します。
3. **Environment variables** に `DISCORD_TOKEN`、`TRANSLATION_PROVIDER`、選んだ翻訳プロバイダのAPIキーを登録してデプロイします。

プロバイダ名と必須キーは上の「プロバイダの設定」表を参照してください。Portainerはこのリポジトリの `.env` を自動では読み込まないため、値はStackの環境変数として設定します。`bot-config` 名前付きボリュームが設定ファイルを保持し、コンテナを更新しても設定は残ります。

新しいイメージを反映する場合はStackの **Pull and redeploy** を実行します。Docker Composeでは `docker compose pull && docker compose up -d` を実行してください。

新しい `bot-config` ボリュームはイメージ内の `config.example.yml` から初期化されます。既存の `config.yml` を使う場合は、初回起動前に内容をボリュームへコピーしてください。`.env` や実際の設定ファイルをイメージに焼き込まないでください。

Discord Developer PortalでMessage Content Intentを有効にしてください。Botには対象チャンネルの閲覧、メッセージ履歴の閲覧、メッセージ送信権限が必要です。フォーラムへの翻訳投稿にはフォーラム投稿権限も必要です。

## ライセンス

このプロジェクトは [GNU General Public License v3.0 only](LICENSE) の下で公開しています。

## systemd

サービスファイルは `/opt/discord_translator_bot` に配置する前提です。`.env` と `config.yml` を同ディレクトリに作り、バイナリをビルドしてからサービスを有効にしてください。実行ユーザー `discordbot` がバイナリ、設定ファイル、作業ディレクトリを読み取れる必要があります。
