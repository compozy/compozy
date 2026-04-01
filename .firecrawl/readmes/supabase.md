![](https://user-images.githubusercontent.com/8291514/213727234-cda046d6-28c6-491a-b284-b86c5cede25d.png#gh-light-mode-only)![](https://user-images.githubusercontent.com/8291514/213727225-56186826-bee8-43b5-9b15-86e839d89393.png#gh-dark-mode-only)

\# Supabase

\[Supabase\](https://supabase.com) is the Postgres development platform. We're building the features of Firebase using enterprise-grade open source tools.

\- \[x\] Hosted Postgres Database. \[Docs\](https://supabase.com/docs/guides/database)
\- \[x\] Authentication and Authorization. \[Docs\](https://supabase.com/docs/guides/auth)
\- \[x\] Auto-generated APIs.
 \- \[x\] REST. \[Docs\](https://supabase.com/docs/guides/api)
 \- \[x\] GraphQL. \[Docs\](https://supabase.com/docs/guides/graphql)
 \- \[x\] Realtime subscriptions. \[Docs\](https://supabase.com/docs/guides/realtime)
\- \[x\] Functions.
 \- \[x\] Database Functions. \[Docs\](https://supabase.com/docs/guides/database/functions)
 \- \[x\] Edge Functions \[Docs\](https://supabase.com/docs/guides/functions)
\- \[x\] File Storage. \[Docs\](https://supabase.com/docs/guides/storage)
\- \[x\] AI + Vector/Embeddings Toolkit. \[Docs\](https://supabase.com/docs/guides/ai)
\- \[x\] Dashboard

!\[Supabase Dashboard\](https://raw.githubusercontent.com/supabase/supabase/master/apps/www/public/images/github/supabase-dashboard.png)

Watch "releases" of this repo to get notified of major updates.

``

\## Documentation

For full documentation, visit \[supabase.com/docs\](https://supabase.com/docs)

To see how to Contribute, visit \[Getting Started\](./DEVELOPERS.md)

\## Community & Support

\- \[Community Forum\](https://github.com/supabase/supabase/discussions). Best for: help with building, discussion about database best practices.
\- \[GitHub Issues\](https://github.com/supabase/supabase/issues). Best for: bugs and errors you encounter using Supabase.
\- \[Email Support\](https://supabase.com/docs/support#business-support). Best for: problems with your database or infrastructure.
\- \[Discord\](https://discord.supabase.com). Best for: sharing your applications and hanging out with the community.

\## How it works

Supabase is a combination of open source tools. We’re building the features of Firebase using enterprise-grade, open source products. If the tools and communities exist, with an MIT, Apache 2, or equivalent open license, we will use and support that tool. If the tool doesn't exist, we build and open source it ourselves. Supabase is not a 1-to-1 mapping of Firebase. Our aim is to give developers a Firebase-like developer experience using open source tools.

\*\*Architecture\*\*

Supabase is a \[hosted platform\](https://supabase.com/dashboard). You can sign up and start using Supabase without installing anything.
You can also \[self-host\](https://supabase.com/docs/guides/hosting/overview) and \[develop locally\](https://supabase.com/docs/guides/local-development).

!\[Architecture\](apps/docs/public/img/supabase-architecture.svg)

\- \[Postgres\](https://www.postgresql.org/) is an object-relational database system with over 30 years of active development that has earned it a strong reputation for reliability, feature robustness, and performance.
\- \[Realtime\](https://github.com/supabase/realtime) is an Elixir server that allows you to listen to PostgreSQL inserts, updates, and deletes using websockets. Realtime polls Postgres' built-in replication functionality for database changes, converts changes to JSON, then broadcasts the JSON over websockets to authorized clients.
\- \[PostgREST\](http://postgrest.org/) is a web server that turns your PostgreSQL database directly into a RESTful API.
\- \[GoTrue\](https://github.com/supabase/gotrue) is a JWT-based authentication API that simplifies user sign-ups, logins, and session management in your applications.
\- \[Storage\](https://github.com/supabase/storage-api) a RESTful API for managing files in S3, with Postgres handling permissions.
\- \[pg\_graphql\](http://github.com/supabase/pg\_graphql/) a PostgreSQL extension that exposes a GraphQL API.
\- \[postgres-meta\](https://github.com/supabase/postgres-meta) is a RESTful API for managing your Postgres, allowing you to fetch tables, add roles, and run queries, etc.
\- \[Kong\](https://github.com/Kong/kong) is a cloud-native API gateway.

\#### Client libraries

Our approach for client libraries is modular. Each sub-library is a standalone implementation for a single external system. This is one of the ways we support existing tools.

| Language | Client | Feature-Clients (bundled in Supabase client) |
| --- | --- | --- |
|  | Supabase | [PostgREST](https://github.com/postgrest/postgrest) | [GoTrue](https://github.com/supabase/gotrue) | [Realtime](https://github.com/supabase/realtime) | [Storage](https://github.com/supabase/storage-api) | Functions |
| ⚡️ Official ⚡️ |
| JavaScript (TypeScript) | [supabase-js](https://github.com/supabase/supabase-js) | [postgrest-js](https://github.com/supabase/supabase-js/tree/master/packages/core/postgrest-js) | [auth-js](https://github.com/supabase/supabase-js/tree/master/packages/core/auth-js) | [realtime-js](https://github.com/supabase/supabase-js/tree/master/packages/core/realtime-js) | [storage-js](https://github.com/supabase/supabase-js/tree/master/packages/core/storage-js) | [functions-js](https://github.com/supabase/supabase-js/tree/master/packages/core/functions-js) |
| Flutter | [supabase-flutter](https://github.com/supabase/supabase-flutter) | [postgrest-dart](https://github.com/supabase/postgrest-dart) | [gotrue-dart](https://github.com/supabase/gotrue-dart) | [realtime-dart](https://github.com/supabase/realtime-dart) | [storage-dart](https://github.com/supabase/storage-dart) | [functions-dart](https://github.com/supabase/functions-dart) |
| Swift | [supabase-swift](https://github.com/supabase/supabase-swift) | [postgrest-swift](https://github.com/supabase/supabase-swift/tree/main/Sources/PostgREST) | [auth-swift](https://github.com/supabase/supabase-swift/tree/main/Sources/Auth) | [realtime-swift](https://github.com/supabase/supabase-swift/tree/main/Sources/Realtime) | [storage-swift](https://github.com/supabase/supabase-swift/tree/main/Sources/Storage) | [functions-swift](https://github.com/supabase/supabase-swift/tree/main/Sources/Functions) |
| Python | [supabase-py](https://github.com/supabase/supabase-py) | [postgrest-py](https://github.com/supabase/postgrest-py) | [gotrue-py](https://github.com/supabase/gotrue-py) | [realtime-py](https://github.com/supabase/realtime-py) | [storage-py](https://github.com/supabase/storage-py) | [functions-py](https://github.com/supabase/functions-py) |
| 💚 Community 💚 |
| C# | [supabase-csharp](https://github.com/supabase-community/supabase-csharp) | [postgrest-csharp](https://github.com/supabase-community/postgrest-csharp) | [gotrue-csharp](https://github.com/supabase-community/gotrue-csharp) | [realtime-csharp](https://github.com/supabase-community/realtime-csharp) | [storage-csharp](https://github.com/supabase-community/storage-csharp) | [functions-csharp](https://github.com/supabase-community/functions-csharp) |
| Go | - | [postgrest-go](https://github.com/supabase-community/postgrest-go) | [gotrue-go](https://github.com/supabase-community/gotrue-go) | - | [storage-go](https://github.com/supabase-community/storage-go) | [functions-go](https://github.com/supabase-community/functions-go) |
| Java | - | - | [gotrue-java](https://github.com/supabase-community/gotrue-java) | - | [storage-java](https://github.com/supabase-community/storage-java) | - |
| Kotlin | [supabase-kt](https://github.com/supabase-community/supabase-kt) | [postgrest-kt](https://github.com/supabase-community/supabase-kt/tree/master/Postgrest) | [auth-kt](https://github.com/supabase-community/supabase-kt/tree/master/Auth) | [realtime-kt](https://github.com/supabase-community/supabase-kt/tree/master/Realtime) | [storage-kt](https://github.com/supabase-community/supabase-kt/tree/master/Storage) | [functions-kt](https://github.com/supabase-community/supabase-kt/tree/master/Functions) |
| Ruby | [supabase-rb](https://github.com/supabase-community/supabase-rb) | [postgrest-rb](https://github.com/supabase-community/postgrest-rb) | - | - | - | - |
| Rust | - | [postgrest-rs](https://github.com/supabase-community/postgrest-rs) | - | - | - | - |
| Godot Engine (GDScript) | [supabase-gdscript](https://github.com/supabase-community/godot-engine.supabase) | - | - | - | - | - |

\## Badges

!\[Made with Supabase\](./apps/www/public/badge-made-with-supabase.svg)

\`\`\`md
\[!\[Made with Supabase\](https://supabase.com/badge-made-with-supabase.svg)\](https://supabase.com)
\`\`\`

\`\`\`html
[![Made with Supabase](https://supabase.com/badge-made-with-supabase.svg)](https://supabase.com/)
\`\`\`

!\[Made with Supabase (dark)\](./apps/www/public/badge-made-with-supabase-dark.svg)

\`\`\`md
\[!\[Made with Supabase\](https://supabase.com/badge-made-with-supabase-dark.svg)\](https://supabase.com)
\`\`\`

\`\`\`html
[![Made with Supabase](https://supabase.com/badge-made-with-supabase-dark.svg)](https://supabase.com/)
\`\`\`

\## Translations

\- \[Arabic \| العربية\](/i18n/README.ar.md)
\- \[Albanian / Shqip\](/i18n/README.sq.md)
\- \[Bangla / বাংলা\](/i18n/README.bn.md)
\- \[Bulgarian / Български\](/i18n/README.bg.md)
\- \[Catalan / Català\](/i18n/README.ca.md)
\- \[Croatian / Hrvatski\](/i18n/README.hr.md)
\- \[Czech / čeština\](/i18n/README.cs.md)
\- \[Danish / Dansk\](/i18n/README.da.md)
\- \[Dutch / Nederlands\](/i18n/README.nl.md)
\- \[English\](https://github.com/supabase/supabase)
\- \[Estonian / eesti keel\](/i18n/README.et.md)
\- \[Finnish / Suomalainen\](/i18n/README.fi.md)
\- \[French / Français\](/i18n/README.fr.md)
\- \[German / Deutsch\](/i18n/README.de.md)
\- \[Greek / Ελληνικά\](/i18n/README.el.md)
\- \[Gujarati / ગુજરાતી\](/i18n/README.gu.md)
\- \[Hebrew / עברית\](/i18n/README.he.md)
\- \[Hindi / हिंदी\](/i18n/README.hi.md)
\- \[Hungarian / Magyar\](/i18n/README.hu.md)
\- \[Nepali / नेपाली\](/i18n/README.ne.md)
\- \[Indonesian / Bahasa Indonesia\](/i18n/README.id.md)
\- \[Italiano / Italian\](/i18n/README.it.md)
\- \[Japanese / 日本語\](/i18n/README.jp.md)
\- \[Korean / 한국어\](/i18n/README.ko.md)
\- \[Lithuanian / lietuvių\](/i18n/README.lt.md)
\- \[Latvian / latviski\](/i18n/README.lv.md)
\- \[Malay / Bahasa Malaysia\](/i18n/README.ms.md)
\- \[Norwegian (Bokmål) / Norsk (Bokmål)\](/i18n/README.nb.md)
\- \[Persian / فارسی\](/i18n/README.fa.md)
\- \[Polish / Polski\](/i18n/README.pl.md)
\- \[Portuguese / Português\](/i18n/README.pt.md)
\- \[Portuguese (Brazilian) / Português Brasileiro\](/i18n/README.pt-br.md)
\- \[Romanian / Română\](/i18n/README.ro.md)
\- \[Russian / Pусский\](/i18n/README.ru.md)
\- \[Serbian / Srpski\](/i18n/README.sr.md)
\- \[Sinhala / සිංහල\](/i18n/README.si.md)
\- \[Slovak / slovenský\](/i18n/README.sk.md)
\- \[Slovenian / Slovenščina\](/i18n/README.sl.md)
\- \[Spanish / Español\](/i18n/README.es.md)
\- \[Simplified Chinese / 简体中文\](/i18n/README.zh-cn.md)
\- \[Swedish / Svenska\](/i18n/README.sv.md)
\- \[Thai / ไทย\](/i18n/README.th.md)
\- \[Traditional Chinese / 繁體中文\](/i18n/README.zh-tw.md)
\- \[Turkish / Türkçe\](/i18n/README.tr.md)
\- \[Ukrainian / Українська\](/i18n/README.uk.md)
\- \[Vietnamese / Tiếng Việt\](/i18n/README.vi-vn.md)
\- \[List of translations\](/i18n/languages.md)