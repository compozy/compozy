import { createFileRoute } from "@tanstack/react-router";

import { EDITION, VERSION } from "../manual/chapters";
import { Applications } from "../manual/foundations/applications";
import { Audience } from "../manual/foundations/audience";
import { Color } from "../manual/foundations/color";
import { Elements } from "../manual/foundations/elements";
import { Essence } from "../manual/foundations/essence";
import { Form } from "../manual/foundations/form";
import { Logo } from "../manual/foundations/logo";
import { Messaging } from "../manual/foundations/messaging";
import { Story } from "../manual/foundations/story";
import { Tokens } from "../manual/foundations/tokens";
import { Type } from "../manual/foundations/type";
import { Voice } from "../manual/foundations/voice";
import { chapterHead } from "../manual/head";
import { ChapterPage } from "../manual/parts/chapter-page";

export const Route = createFileRoute("/")({
  head: () =>
    chapterHead("", "Compozy brand manual: foundations, symbol, colour, type, voice and tokens."),
  component: FoundationsChapter,
});

function FoundationsChapter() {
  return (
    <ChapterPage
      slug=""
      front
      title={
        <>
          A manual for a brand that <em>keeps running</em>.
        </>
      }
      intro={`Everything Compozy looks and sounds like, in one place: the symbol, the system around it, and the rules that keep every piece consistent. Edition ${VERSION}, ${EDITION}.`}
    >
      <Essence />
      <Audience />
      <Story />
      <Logo />
      <Color />
      <Type />
      <Elements />
      <Form />
      <Voice />
      <Messaging />
      <Applications />
      <Tokens />
    </ChapterPage>
  );
}
