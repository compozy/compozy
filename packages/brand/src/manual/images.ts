// Every generated raster used by the manual. Files: public/brand/img/<area>/<id>.jpg (≤1600 px).
// Only the files a ported chapter renders are copied into public/ (S2: kit/laptop-site, kit/monitor-app);
// chapters ported later copy theirs from ~/Dev/compozy/new-branding/public/assets/img.
// Prompts: prompts/<area>/<id>.md. Generated with Codex image_gen (gpt-image), reviewed on a contact sheet.
// Generated people never stand for real team members, partners or users.

export interface Img {
  id: string;
  area: "photo" | "world" | "kit";
  alt: string;
  ratio: string;
  caption: string;
}

const img = (area: Img["area"], id: string, ratio: string, alt: string, caption: string): Img => ({
  id,
  area,
  ratio,
  alt,
  caption,
});

export const images: Img[] = [
  // photography direction (chapter 06)
  img(
    "photo",
    "operator-night",
    "4/5",
    "Engineer at a desk late at night, face lit by a monitor",
    "The operator: one person, real work, screen light."
  ),
  img(
    "photo",
    "pair-review",
    "3/2",
    "Two engineers reviewing work over a shoulder",
    "Two people, one decision. The agent did the work; a person approves it."
  ),
  img(
    "photo",
    "hands-detail",
    "4/5",
    "Hands on a mechanical keyboard, close detail",
    "Detail: hands and tools, never a readable screen."
  ),
  img(
    "photo",
    "meetup-room",
    "16/9",
    "A small technical meetup seen from the back of the room",
    "Community: wide, from the back, people listening."
  ),
  img(
    "photo",
    "laptop-closed",
    "3/2",
    "A closed laptop on a desk at night with a small status light on",
    "The promise as an image: the laptop is closed, the work keeps going."
  ),
  img(
    "photo",
    "portrait-window",
    "4/5",
    "Portrait of an engineer by a window in cold daylight",
    "Portrait: cold daylight, direct, unposed."
  ),
  img(
    "photo",
    "avoid-stock",
    "3/2",
    "Counter-example: glossy stock photo of a robot hand touching a glowing brain",
    "Counter-example: the AI stock image we never use."
  ),
  // the brand on real materials (chapters 02 and 11)
  img(
    "world",
    "tee-front",
    "4/5",
    "Black t-shirt with the Compozy symbol printed on the chest",
    "Tee, front: symbol in five-colour screen print."
  ),
  img(
    "world",
    "hoodie-back",
    "4/5",
    "Black hoodie with a large dot-field print on the back",
    "Hoodie, back: the field visual as a fine screen print."
  ),
  img(
    "world",
    "cap",
    "1/1",
    "Black cap with a small embroidered symbol",
    "Cap: embroidered symbol, one colour."
  ),
  img(
    "world",
    "tote",
    "4/5",
    "Natural tote bag with the symbol printed in one ink colour",
    "Tote: one-colour ink on natural cotton."
  ),
  img(
    "world",
    "laptop-stickers",
    "3/2",
    "Laptop lid with Compozy die-cut stickers",
    "Stickers on a working laptop."
  ),
  img(
    "world",
    "enamel-pin",
    "1/1",
    "Enamel pin of the loop symbol on a dark fabric",
    "Enamel pin: gradient as a soft enamel fade."
  ),
  img(
    "world",
    "poster-wall",
    "4/5",
    "Three A3 posters with the swarm visual pasted on a concrete wall",
    "Posters: the A3 template, wheat-pasted on concrete."
  ),
  img(
    "world",
    "mug",
    "1/1",
    "Matte black mug with the symbol",
    "Mug: matte black, symbol small and off-centre."
  ),
  img(
    "world",
    "notebook",
    "4/5",
    "Dark notebook with the symbol debossed on the cover",
    "Notebook: blind deboss, no ink."
  ),
  img(
    "world",
    "keycap",
    "1/1",
    "Artisan keycap with the loop symbol",
    "Keycap: the symbol in resin."
  ),
  img(
    "world",
    "stage-screen",
    "16/9",
    "Conference stage with a large screen showing the symbol",
    "Stage: the symbol at architectural scale."
  ),
  img(
    "world",
    "lanyard",
    "4/5",
    "Event badge and lanyard with Compozy branding on a table",
    "Badge and lanyard for an event."
  ),
  // finished pieces in context (chapters 07, 08, 10, 11)
  img(
    "kit",
    "phone-feed",
    "4/5",
    "Phone showing a Compozy post in a social feed",
    "X feed with the release post."
  ),
  img(
    "kit",
    "laptop-site",
    "3/2",
    "Laptop on a desk showing the Compozy site hero",
    "Site hero on a laptop."
  ),
  img(
    "kit",
    "monitor-app",
    "16/9",
    "Desktop monitor showing the Compozy desktop app",
    "The desktop app as the brand’s main surface."
  ),
  img(
    "kit",
    "billboard",
    "16/9",
    "Large billboard with a Compozy brand ad at dusk",
    "Billboard: one line, one figure."
  ),
  img(
    "kit",
    "bus-shelter",
    "4/5",
    "Bus shelter poster with a Compozy ad",
    "Street poster, portrait."
  ),
  img(
    "kit",
    "slide-talk",
    "16/9",
    "Speaker on stage presenting a Compozy slide",
    "Slides at a meetup."
  ),
  img(
    "kit",
    "business-cards",
    "3/2",
    "Compozy business cards on a dark surface",
    "Business cards: ink front, gradient edge."
  ),
  img(
    "kit",
    "rollup",
    "4/5",
    "Roll-up banner at an event entrance",
    "Roll-up at an event entrance."
  ),
];

export const imgSrc = (i: Img) => `/brand/img/${i.area}/${i.id}.jpg`;
export const imageById = (id: string): Img => {
  const i = images.find(x => x.id === id);
  if (!i) throw new Error(`unknown image ${id}`);
  return i;
};
