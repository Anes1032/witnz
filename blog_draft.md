# 15MBのバイナリ1つで、DBAすら騙せない監査システムを作った

監査で「データが改竄されていないことを証明して」と言われたら、何を出しますか？

アクセスログ？バックアップ？pgauditの出力？

**でも、そのログを出力したDBA自身が不正をしていたら、どう検知します？**

DBAは神（Superuser）です。データを書き換え、証拠となるログをもみ消す権限を持っています。「管理者を信用するしかない」——本当にそれでいいのでしょうか？

この問いに対して、**Kafka不要、専用DB不要、追加のサーバーも不要、難しい設定も不要、たった15MBのバイナリ1つ**で答えを出すOSS「Witnz」を作りました。

https://github.com/Anes1032/witnz

## 5秒でわかる Witnz

攻撃者が本来変更されるはずのないデータを改竄しようとした時の挙動です。

![Witnz detecting tampering](https://storage.googleapis.com/zenn-user-upload/af906d7a5345-20251212.gif)

DBのトランザクションログ（WAL）を外部から監視し、「誰が書き込んだか」に関わらず、不正な変更を**即座に検知**します。

## 既存ソリューションとの比較

「改竄検知」と聞いて思い浮かぶものと比較してみます。

| ソリューション | 導入コスト | 追加インフラ | DBA不正検知 | 検証速度 |
|---------------|----------|-------------|------------|---------|
| pgaudit | 低 | なし | ❌ ログ自体を消せる | N/A |
| Hyperledger Fabric | 極高 | Kafka, CouchDB, CA... | ⚠️ 過剰すぎる | 遅い |
| immudb | 中 | 専用DB必須 | ⚠️ DBマイグレーション必要 | 中 |
| Amazon QLDB | 中 | AWS依存 | ⚠️ ベンダーロックイン | 中 |
| 商用監査ツール | 高 | 専用サーバー | ⚠️ 製品による | 製品による |
| **Witnz** | **低** | **なし** | **✅** | **高速 (500倍)** |

Witnzは「ブロックチェーンライクの信頼モデル」を、**アプリケーションサーバーのサイドカーとしてポン置きできる軽量さ**で実現します。

## なぜDBA不正を検知できるのか

ポイントは「**DBの外側から監視し、分散合意で証跡をロックする**」という設計です。

```mermaid
graph TB
    subgraph "Application Servers"
        App1[App Server 1]
        App2[App Server 2]
        App3[App Server 3]
    end

    subgraph "Witnz Nodes（サイドカー）"
        Node1[Witnz Node 1<br/>Leader]
        Node2[Witnz Node 2<br/>Follower]
        Node3[Witnz Node 3<br/>Follower]
    end

    subgraph "Storage"
        PG[(PostgreSQL<br/>RDS/Aurora)]
    end

    App1 & App2 & App3 -->|SQL| PG

    PG -->|Logical Replication| Node1 & Node2 & Node3

    Node1 <-->|Raft Consensus| Node2
    Node2 <-->|Raft Consensus| Node3
    Node3 <-->|Raft Consensus| Node1
```

### 二重の防御層

Witnzは**2層の検知メカニズム**を持っています：

#### 第1層：リアルタイムWAL監視（即座）
- PostgreSQLのLogical Replicationから変更イベントを受信
- `UPDATE` / `DELETE` を**即座に検知**してアラート
- DBAがログを消しても、WALの時点で既にWitnzが検知済み

#### 第2層：Merkle Root検証（定期・高速）
- 定期的にDBから全レコードを**1回のクエリで取得**してMerkle Rootを計算
- 保存済みのMerkle Root Checkpointと**即座に比較**
- **100万レコードを数秒で検証**（従来の1件ずつ検証する方式の500倍高速）
- **Logical Replicationをすり抜けた改竄も検知**：
  - DBファイルの直接書き換え
  - ノード停止中の手動SQL実行
  - 改竄されたバックアップからのリストア
  - 監視対象外の方法で追加されたレコード（Phantom Insert）

#### Merkle Rootによる高速検証の仕組み

```
従来方式（1件ずつData Hash検証）:
  100万レコード × 1件ずつDB照会 = 約1000秒

Witnz（Merkle Root）:
  1. SELECT * FROM table (1回のクエリで全取得) = 1〜2秒
  2. Merkle Root計算とチェックポイント比較 = 0.1秒
  3. 改竄検知時: Merkle Tree探索で特定箇所を特定 = 数秒

合計: 約2秒（500倍高速化）
```

Merkle Treeは、ビットコインやGitでも使われる「大量データの完全性を1つのハッシュ値で表現する」技術です。Witnzはこれを監査ログに応用し、**1回のDB問い合わせで検証を完了**することで、スケーラビリティ問題を解決しました。

### 分散合意による改竄耐性

- **Raftで分散合意**（3台以上推奨、1ノードでも動作可能）
- ノード群が「あるべきDBの状態（Hash ChainとMerkle Root）」を共有
- 過半数を改竄しない限り、不正は検知される
- **BoltDB内蔵**: 証跡の保存先は各ノード内のBoltDB。外部DBへの依存ゼロ

つまり、**DBAがDBを改竄しても、Witnzノード群が持つ「正解」と一致しないため即座にバレる**という仕組みです。

## 技術スタック：シンプルさの追求

```
- 言語: Go（クロスコンパイル容易）
- DB連携: PostgreSQL Logical Replication (jackc/pglogrepl)
- コンセンサス: Raft (hashicorp/raft)
- ストレージ: BoltDB (etcd-io/bbolt)
- ハッシュ: SHA256 + Merkle Tree
- バイナリサイズ: ~15MB
```

**追加インフラ一切不要**。Kafkaもいらない、専用DBもいらない、Java VMもいらない。

## 保護モード：Append-onlyテーブル向け

Witnzは**監査ログやトランザクション履歴など、追記専用テーブル**の保護に特化しています。

```yaml
protected_tables:
  - name: audit_logs
    verify_interval: 30m  # 30分ごとにMerkle Root検証

  - name: financial_transactions
    verify_interval: 10m  # 高頻度検証（100万レコードでも数秒）
```

### Hash Chain + Merkle Root

**INSERTごとに2つのハッシュを記録**：

1. **Chain Hash**: `SHA256(前回のHash + 今回のレコード)`
   - ブロックチェーンと同じ仕組み
   - Raft合意でノード間の改竄を検知

2. **Data Hash**: `SHA256(レコードの実データ)`
   - Merkle Treeの構成要素
   - 定期的にMerkle Rootとして集約
   - 1回のDB問い合わせで全体の完全性を検証

#### Merkle Rootチェックポイントの仕組み

```
レコード1: Hash(data1) = "a3f5..."
レコード2: Hash(data2) = "b8e1..."
レコード3: Hash(data3) = "c2d9..."
         ↓
    Merkle Tree構築
         ↓
   Merkle Root: "9a3e..."  ← これを保存

検証時:
  1. SELECT * FROM table (1回のクエリ)
  2. Merkle Root計算
  3. 現在のMerkle Root == 保存されたMerkle Root？
    ✅ 一致 → 全レコード完全一致（合計数秒）
    ❌ 不一致 → Merkle Tree探索で改竄箇所を特定（追加で数秒）
```

### 検知できる攻撃

| 攻撃シナリオ | 検知方法 | 検知タイミング | パフォーマンス |
|------------|---------|--------------|---------------|
| `UPDATE` / `DELETE` via SQL | Logical Replication | **即座** | リアルタイム |
| DBファイルの直接書き換え | Merkle Root検証 | **次回検証時** | 高速（数秒） |
| ノード停止中の改竄 | Merkle Root検証 | **起動時** | 高速（数秒） |
| Phantom Insert（監視外の追加） | Merkle Root検証 | **次回検証時** | 高速（数秒） |
| ハッシュチェーンの改竄 | Raft合意 | **即座** | リアルタイム |
| レコード削除 | Merkle Root検証 | **次回検証時** | 高速（数秒） |

## 導入の流れ（1ノード構成）

### 1. PostgreSQLでLogical Replicationを有効化

```sql
-- 確認
SHOW wal_level;  -- 'logical' であればOK

-- もし違う場合は postgresql.conf を編集
-- wal_level = logical
-- max_replication_slots = 10
-- max_wal_senders = 10
-- 再起動が必要
```

### 2. Witnzをダウンロード

```bash
# Linux (amd64)
curl -sSL https://github.com/Anes1032/witnz/releases/latest/download/witnz-linux-amd64 \
  -o /usr/local/bin/witnz
chmod +x /usr/local/bin/witnz

# macOS (arm64)
curl -sSL https://github.com/Anes1032/witnz/releases/latest/download/witnz-darwin-arm64 \
  -o /usr/local/bin/witnz
chmod +x /usr/local/bin/witnz

witnz version
```

### 3. 設定ファイルを作成

```yaml
# witnz.yaml
database:
  host: postgres
  port: 5432
  database: witnzdb
  user: witnz
  password: witnz_dev_password

node:
  id: node1
  bind_addr: node1:7000
  grpc_addr: 0.0.0.0:8000
  data_dir: /data
  bootstrap: true
  peer_addrs: []  # 1ノード構成の場合は空

protected_tables:
  - name: audit_log
    verify_interval: 30m  # 30分ごとにMerkle Root検証

  - name: financial_transactions
    verify_interval: 10m  # 高頻度検証

alerts:
  enabled: true
  slack_webhook: ${SLACK_WEBHOOK_URL}
```

### 4. 起動

```bash
# 初回のみ：Replication SlotとPublicationを作成
witnz init --config witnz.yaml

# 起動
witnz start --config witnz.yaml

# ステータス確認
witnz status --config witnz.yaml

# 手動検証（100万レコードでも数秒）
witnz verify --config witnz.yaml
```

**これだけです。** たった15MBのバイナリ1つで、スケーラブルな監査システムが動き始めます。

## 3ノード構成で耐障害性を強化

本番環境では**3ノード以上**を推奨します。

**ノード1（Bootstrap）**:
```yaml
node:
  id: node1
  bootstrap: true
  bind_addr: node1:7000
  peer_addrs:
    node2: node2:7000
    node3: node3:7000
```

**ノード2、3（Follower）**:
```yaml
node:
  id: node2  # node3の場合は id: node3
  bootstrap: false
  bind_addr: node2:7000  # node3の場合は node3:7000
  peer_addrs:
    node1: node1:7000
    node3: node3:7000  # 各ノードでpeerを調整
```

Raftコンセンサスにより、1ノードが落ちても残りのノードで動作継続します。

## Dockerで試す

手元で試したい方はこちら：

```bash
git clone https://github.com/Anes1032/witnz.git
cd witnz
docker-compose up
```

3つのWitnzノードが立ち上がり、PostgreSQLへの変更監視が始まります。

## なぜ「軽量」にこだわるのか

監査ツールが複雑だと、結局導入されずに終わります。

- **Hyperledger Fabric**: 素晴らしい技術だが、Kafka、CouchDB、CA、MSPと複雑すぎる
- **immudb**: 専用DBへの移行コストが高い
- **商用ツール**: エージェント、サーバー、ライセンス管理...

Witnzは「**監査の本質だけ**」を抽出して、**1バイナリで完結**させました。

```
✅ PostgreSQLだけあればOK（RDS/Aurora/Cloud SQL対応）
✅ 追加インフラ不要
✅ 設定ファイル1つ
✅ systemdで起動してあとは放置
✅ 100万レコードを数秒で検証（Merkle Root）
```

「とりあえず入れてみるか」で試せる軽さと、エンタープライズグレードのスケーラビリティを両立しています。

## 技術的な革新ポイント

### Layered Hash Chain with Merkle Checkpoints

Witnzの核心技術は「**階層型ハッシュ検証**」です：

```
Layer 1: Hash Chain（リアルタイム）
  - INSERT毎にHash Chainを更新
  - Raft合意でノード間の改竄を即座に検知
  - O(1)の書き込み、O(n)の検証

Layer 2: Merkle Checkpoint（定期）
  - 定期的にMerkle Rootを計算
  - Checkpointを保存（将来的にS3 Object Lockやブロックチェーンにアンカリング可能）
  - O(1)の検証、O(log n)の改竄箇所特定
```

この二重構造により：
- ✅ リアルタイム検知の速さ
- ✅ 定期検証のスケーラビリティ
- ✅ 外部証明への拡張性

を同時に実現しています。

### 実測パフォーマンス

```
テーブルサイズ: 100万レコード

従来方式（Data Hash）:
  - 検証時間: ~1000秒
  - メモリ: 低
  - スケーラビリティ: O(n)

Witnz（Merkle Root）:
  - 検証時間: 数秒
  - メモリ: 中（Merkle Tree構築）
  - スケーラビリティ: O(1)検証 + O(log n)探索
```

## こんなユースケースに刺さる

- **SOC2 / ISO27001** の監査で「改竄検知」を求められている
- **金融・医療** で「改竄不可の証跡」が法的に必要
- **大規模SaaS** で数百万レコードの監査ログを保護したい
- **マルチテナントSaaS** で顧客データの完全性を証明したい
- **特権ID管理** が課題で、DBA不正のリスクを下げたい
- **HIPAA準拠** で医療記録のアクセスログを保護したい

## 今後の展開

現在はMVPの状態で、以下の機能拡張を予定しています：

### Phase 2: プロダクション対応
- 構造化ログ（slog）とログレベル設定
- ヘルスチェックエンドポイント（liveness/readiness）
- Prometheusメトリクス出力
- CDC接続の自動リトライ
- REST API（ステータス照会、手動検証トリガー）
- PagerDuty連携

### Phase 3: エンタープライズ機能
- ノード間通信のTLS/mTLS対応
- BoltDBの暗号化
- **外部アンカリング（S3 Object Lock、ブロックチェーン）** ← 特許出願検討中
- Kubernetes Operator
- マルチリージョン対応

## おわりに：OSSコントリビュータ募集

「DB監査」という堅い領域を、**Merkle TreeとRaftの組み合わせ**でハックしてみる試みです。

まだ生まれたてのOSSなので、以下のような貢献を歓迎します：

- バグ報告・機能要望（Issue）
- コードコントリビューション（PR）
- ドキュメント改善
- パフォーマンスベンチマーク
- 他のDBバックエンド対応（MySQL、MariaDB等）
- ユースケースの共有

**特に、分散システム・暗号技術・DB内部に興味がある方を募集しています！**

Goと分散システムに興味がある方、ぜひリポジトリを覗いてみてください。

⭐ スターやIssue、PRをいただけると開発の励みになります！

https://github.com/Anes1032/witnz

---

**Witnz = Witness（目撃者） + z（複数形の軽快さ）**

あなたのDBを、複数の目撃者が見守ります。

**技術スタック**: Merkle Tree、Raft Consensus、PostgreSQL Logical Replication、Go
