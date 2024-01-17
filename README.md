# WaitingRoom

WaitingRoomは、高負荷なアクセスがあった際に、アクセスを制御し、システムへの負担を分散するためのツールです。設定した閾値を超えるアクセスがあった際に、一時的に待機室に誘導し、アクセスを順番に処理します。

## 機能

- リクエストの監視と制御
- 順番待ちクライアントの管理
- 負荷分散
- OpenTelemetryを利用したトレース情報の収集

## 使用方法

本システムはコマンドラインから操作します。基本的なコマンドは以下の通りです。

```bash
# コンフィグファイルを指定してWaitingRoomを起動
waitingroom --config your_config.toml
```

## 設定ファイル

コンフィグファイルでは以下のオプションを設定できます。ファイルはTOML形式で記述することを想定しています。

```toml
# ログのレベルを指定します。
# 利用可能な値: debug, info, warn, error
log_level = "info"

# サーバーのリスナーアドレスを指定します。
listener = "localhost:18080"

# 公開URLのホストを指定します。
public_host = "localhost:18080"

# アクセス許可後アクセスできる時間を秒単位で指定します。
permitted_access_sec = 600

# 初回エントリーをDelayさせる秒数を指定します。
entry_delay_sec = 10

# 待合室を有効にしておく時間を秒単位で指定します。
queue_enable_sec = 300

# アクセス許可判定周期を秒単位で指定します。
permit_interval_sec = 60

# アクセス許可する単位を指定します。（PermitIntervalSecあたりPermitUnitNumber許可）
permit_unit_number = 1000

# ローカルメモリキャッシュTTLを秒単位で指定します。
cache_ttl_sec = 20

# ローカルメモリネガティブキャッシュTTLを秒単位で指定します。
negative_cache_ttl_sec = 10

# Slack Api Tokenを指定します。
slack_api_token = "your_slack_api_token"

# Slack Channelを指定します。
slack_channel = "your_slack_channel"

# OpenTelemetryによるトレースを有効にするかどうかを指定します。
enable_otel = false
```

これらのオプションはアプリケーションの振る舞いをカスタマイズするために利用されます。必要に応じて適切な値に設定してください。

## コントリビューション

本プロジェクトにコントリビューションをしていただける場合は、以下の手順に従ってください。

1. リポジトリをフォークする
2. ブランチを作成する (`git checkout -b feature/fooBar`)
3. 変更をコミットする (`git commit -am 'Add some fooBar'`)
4. ブランチにプッシュする (`git push origin feature/fooBar`)
5. 新しいプルリクエストを作成する

## ライセンス

本ソフトウェアは、MITライセンスのもとで公開されています。詳細は[LICENSE](https://github.com/pyama86/waitingroom/LICENSE)を参照してください。

## Author

pyama86
