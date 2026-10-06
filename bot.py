#!/usr/bin/env python3
raise SystemExit("This bot has migrated to Go. Build and run it with Docker or go run .")

# -*- coding: utf-8 -*-
"""
Discord 自動翻訳 Bot
  • A→B チャンネル自動転送
  • 国旗リアクション翻訳（一般ユーザーが書けるチャンネルのみ / 1メッセージ1回）
  • クオータ: 1分あたり最大 MAX_PER_MINUTE 回
  • /translate /detect コマンド
"""

import os, yaml, time, collections, logging, aiohttp, discord
from logging.handlers import RotatingFileHandler
from discord.ext import commands
from discord import app_commands

# ───── config.yml ─────
with open("config.yml", "r", encoding="utf-8") as f:
    cfg = yaml.safe_load(f)

DISCORD_TOKEN  = cfg["discord_token"]
DEEPL_API_KEY  = cfg["deepl_api_key"]
CHANNEL_MAP    = cfg.get("channel_mappings", [])
FLAG_MAP       = cfg.get("flag_map", {})

# 1分あたりの最大翻訳回数（リアクション＋A→B 合算）
MAX_PER_MINUTE = cfg.get("max_per_minute", 30)

# ───── logging ─────
os.makedirs("logs", exist_ok=True)
logger = logging.getLogger("translate-bot")
logger.setLevel(logging.INFO)
handler = RotatingFileHandler("logs/bot.log", maxBytes=5*1024*1024, backupCount=3, encoding="utf-8")
handler.setFormatter(logging.Formatter("%(asctime)s [%(levelname)s] %(message)s"))
logger.addHandler(handler)

# ───── Discord Intents ─────
intents = discord.Intents.default()
intents.message_content = True
intents.messages        = True
intents.reactions       = True
intents.members         = True

bot  = commands.Bot(command_prefix="!", intents=intents, allowed_mentions=discord.AllowedMentions.none())
tree = bot.tree

# ───── DeepL 非同期ラッパ ─────
DEEPL_URL = "https://api-free.deepl.com/v2/translate"
async def deepl(text: str, lang: str) -> str:
    async with aiohttp.ClientSession() as sess:
        async with sess.post(DEEPL_URL, data={"auth_key":DEEPL_API_KEY,"text":text,"target_lang":lang}, timeout=15) as r:
            return (await r.json())["translations"][0]["text"]

# ───── 1メッセージ1回 & クオータ制御 ─────
PROCESSED = set()                    # {(message_id, lang)}
EVENTS    = collections.deque()      # timestamps

def allowed_to_translate(message_id: int, lang: str) -> bool:
    now = time.time()
    # クオータチェック
    while EVENTS and now - EVENTS[0] > 60:
        EVENTS.popleft()
    if len(EVENTS) >= MAX_PER_MINUTE:
        return False
    # 重複チェック
    key = (message_id, lang)
    if key in PROCESSED:
        return False
    # 登録
    PROCESSED.add(key)
    EVENTS.append(now)
    return True

# ───── Ready ─────
@bot.event
async def on_ready():
    await tree.sync()
    logger.info(f"Logged in as {bot.user} ({bot.user.id})")

# ───── A→B 転送 ─────
@bot.event
async def on_message(msg: discord.Message):
    if msg.author.bot:
        return
    for m in CHANNEL_MAP:
        if msg.channel.id == m["source_channel_id"]:
            tgt = bot.get_channel(m["target_channel_id"])
            if tgt and allowed_to_translate(msg.id, "EN"):
                try:
                    text = await deepl(msg.content, "EN")
                    await tgt.send(f"**{msg.author.display_name}**:\n{text}")
                except Exception as e:
                    await tgt.send(f"⚠ 翻訳エラー: {e}")
            break
    await bot.process_commands(msg)

# ───── 国旗リアクション翻訳 ─────
@bot.event
async def on_raw_reaction_add(payload: discord.RawReactionActionEvent):
    emoji_str = str(payload.emoji)
    lang = next((v for k,v in FLAG_MAP.items() if k in emoji_str), None)
    if not lang:
        return

    channel = bot.get_channel(payload.channel_id)
    guild   = bot.get_guild(payload.guild_id)
    if not channel or not guild:
        return

    # @everyone が send_messages できるチャンネルのみ許可
    if not channel.permissions_for(guild.default_role).send_messages:
        return

    try:
        msg = await channel.fetch_message(payload.message_id)
    except Exception as e:
        logger.warning(f"fetch_message failed: {e}")
        return

    if not allowed_to_translate(msg.id, lang):
        return

    try:
        trans = await deepl(msg.content, lang)
        await msg.reply(f"{emoji_str} **{lang}**:\n{trans}", mention_author=False)
    except Exception as e:
        logger.error(f"翻訳失敗: {e}")

# ───── コマンド（簡易） ─────
@tree.command(name="translate", description="テキスト翻訳")
@app_commands.describe(text="本文", target="例: JA")
async def _translate(inter: discord.Interaction, text: str, target: str):
    await inter.response.defer()
    try:
        res = await deepl(text, target.upper())
        await inter.followup.send(f"**→{target.upper()}**\n{res}")
    except Exception as e:
        await inter.followup.send(f"⚠ {e}")

@tree.command(name="detect", description="言語検出")
@app_commands.describe(text="本文")
async def _detect(inter: discord.Interaction, text: str):
    await inter.response.defer()
    code = await deepl(text[:20] or "a", "EN")
    await inter.followup.send(f"Detect: **{code}**")

# ───── Run ─────
bot.run(DISCORD_TOKEN)
