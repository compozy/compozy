import { card, lab } from "../cls";
import { Frame } from "../kit-frame";
import { DoDont } from "../parts/do-dont";
import { Section } from "../parts/section";
import { codeRules } from "./data";
import { Rows } from "./rows";

export function Code() {
  return (
    <Section
      id="code"
      n="07"
      file="ui-card.png"
      title={
        <>
          Our UI as an <em>image</em>
        </>
      }
      bg="black"
      status="proposal"
      intro={
        <p>
          When a feature travels as a picture, on X, LinkedIn or the blog, the picture is the
          product: one screen of the desktop, readable at phone size. Code excerpts stay a secondary
          docs piece. Call backgrounds and wallpapers use Atmosphere.
        </p>
      }
    >
      <div className="grid gap-6 lg:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)]">
        <Frame id="code-card" guides />
        <div className={card}>
          <p className={lab}>Rules</p>
          <Rows rows={codeRules} className="mt-3" />
        </div>
      </div>
      <DoDont
        className="mt-6"
        doContent={
          <p className="m-0 text-[14.5px]">
            UI card: the approval dock, one caption, the lockup. The reader sees what they would
            use.
          </p>
        }
        dontContent={
          <p className="m-0 text-[14.5px] text-(--b-muted)">
            A terminal window with <span className="font-mono">$ compozy …</span> as the hero of a
            feature post. The terminal is the before picture.
          </p>
        }
      />
      <div className="mt-10 grid gap-6 md:grid-cols-2">
        <Frame id="call-background" guides />
        <Frame id="wallpaper" guides />
      </div>
      <p className="mt-4 text-[14px] text-(--b-muted)">
        Call backgrounds export at 1920×1080 PNG; Zoom and Meet mirror the self-view, so the lockup
        reads backwards only to the speaker. Wallpapers export at 2560×1440 PNG and scale cleanly to
        16:9 screens of any size.
      </p>
    </Section>
  );
}
