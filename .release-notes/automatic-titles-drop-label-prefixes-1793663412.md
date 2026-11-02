---
title: Automatic session titles drop label prefixes
type: fix
---

Automatic titles no longer keep a label the model put in front of them, such as `Título sugerido: Localizar resenhas` or `Suggested title: Checkout retry race`. CompozyOS now asks for the title alone, in the conversation's language, and strips these labels in English, Portuguese, Spanish, French, German and Italian. Titles that really contain a colon, such as `Fix title: colon handling`, stay whole.

A label in another language can still pass through, and a title that truly opens with a listed phrase, such as `Session title:`, loses it. Titles already stored are not rewritten.
