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
