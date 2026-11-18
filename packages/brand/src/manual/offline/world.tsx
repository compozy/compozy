// 11 · 07 In the world: generated references of the brand on real materials.
import { imageById, imgSrc } from "../images";
import { GenImage } from "../parts/gen-image";
import { Section } from "../parts/section";
import { world } from "./data";

function WorldImage({ id, ratio, className }: { id: string; ratio: string; className?: string }) {
  const i = imageById(id);
  return (
    <GenImage src={imgSrc(i)} alt={i.alt} caption={i.caption} ratio={ratio} className={className} />
  );
}

export function World() {
  return (
    <Section
      id="world"
      n="07"
      file="assets/img/world/"
      title={
        <>
          The brand on real <em>things</em>
        </>
      }
      bg="black"
      status="proposal"
      intro={
        <p>
          Generated references for the pieces in this chapter: what the tee, the stickers and the
          stage should feel like when they exist. They set direction for vendors; they are not
          photos of products already made.
        </p>
      }
    >
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {world.slice(0, 8).map(id => (
          <WorldImage key={id} id={id} ratio="4/5" />
        ))}
      </div>
      <WorldImage id="stage-screen" ratio="16/9" className="mt-6" />
      <div className="mt-6 grid gap-4 sm:grid-cols-3">
        {["notebook", "keycap", "lanyard"].map(id => (
          <WorldImage key={id} id={id} ratio="4/5" />
        ))}
      </div>
    </Section>
  );
}
