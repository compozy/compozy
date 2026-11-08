// Metadata of every kit template (size, margins, platform-covered bands, art level, print size).
// Chapters 07–12 render template previews from it as marked slots until the kit components and
// /kit/$id arrive with slice S5, which owns this table from then on.
// Columns: id | name | channel | WxH px | level | margin t r b l (%) | unsafe t r b l (%) | print WxH+bleed mm | use

export type KitChannel = "social" | "ads" | "editorial" | "offline" | "product";
/** percentages of width (left/right) and height (top/bottom) */
export interface Insets {
  t: number;
  r: number;
  b: number;
  l: number;
}

export interface KitSpec {
  id: string;
  name: string;
  channel: KitChannel;
  w: number;
  h: number;
  /** art level: 0 type only · 1 field · 2 figure · 3 product */
  level: 0 | 1 | 2 | 3;
  margin: Insets;
  unsafe?: Insets;
  print?: { wMm: number; hMm: number; bleedMm: number };
  use: string;
}

const TABLE = `
x-post | X post | social | 1600x900 | 2 | 11.1 6.25 11.1 6.25 | - | - | Announcements and releases on X
x-quote | X quote card | social | 1600x900 | 1 | 11.1 6.25 11.1 6.25 | - | - | A real, attributed quote from a named person
x-header | X header | social | 1500x500 | 2 | 20 6 20 40 | 10 0 10 0 | - | Profile header on X
x-avatar | X avatar | social | 400x400 | 1 | 14.6 14.6 14.6 14.6 | - | - | Profile picture on X (circle crop)
linkedin-banner | LinkedIn banner (personal) | social | 1584x396 | 2 | 16 5 16 32 | 0 0 0 22 | - | Founder and team profile background
linkedin-cover | LinkedIn cover (company) | social | 1128x191 | 2 | 18 18 18 26 | 0 0 0 20 | - | Company page cover
linkedin-avatar | LinkedIn logo | social | 400x400 | 1 | 14.6 14.6 14.6 14.6 | - | - | Company page logo (square, shown rounded)
linkedin-post | LinkedIn post | social | 1200x1200 | 2 | 7.5 7.5 7.5 7.5 | - | - | Single-image post with context
carousel-cover | Carousel cover | social | 1080x1350 | 2 | 6.7 8.3 6.7 8.3 | - | - | First slide of a LinkedIn document or X image set
carousel-body | Carousel body | social | 1080x1350 | 0 | 6.7 8.3 6.7 8.3 | - | - | One idea per slide, with an artifact
carousel-cta | Carousel CTA | social | 1080x1350 | 2 | 6.7 8.3 6.7 8.3 | - | - | Last slide: one action and the real command
story | Vertical cover | social | 1080x1920 | 2 | 13 8.3 18 8.3 | 13 0 18 0 | - | Vertical video cover, Shorts, Discord event
youtube-banner | YouTube banner | social | 2560x1440 | 2 | 35.3 19.8 35.3 19.8 | 35.3 0 35.3 0 | - | Channel art; safe area 1546×423 centred
youtube-avatar | YouTube avatar | social | 800x800 | 1 | 14.6 14.6 14.6 14.6 | - | - | Channel picture (circle crop)
youtube-thumb | YouTube thumbnail | social | 1280x720 | 3 | 7.5 5 7.5 5 | 0 0 14 0 | - | Video thumbnail, read at 168 px
github-social-preview | GitHub social preview | social | 1280x640 | 2 | 12.5 6.25 12.5 6.25 | - | - | Repository social preview (unfurls in Slack, X, Discord)
github-avatar | GitHub org avatar | social | 500x500 | 1 | 14.6 14.6 14.6 14.6 | - | - | Organisation avatar (rounded square)
readme-header | README header | social | 1280x320 | 2 | 18 30 18 5 | - | - | Top of README.md, light and dark themes
discord-banner | Discord banner | social | 960x540 | 2 | 10 6 10 6 | - | - | Server banner above the channel list
discord-icon | Discord server icon | social | 512x512 | 1 | 14.6 14.6 14.6 14.6 | - | - | Server icon (circle crop)
release-card | Release card | social | 1600x900 | 3 | 11.1 6.25 11.1 6.25 | - | - | Release on X, LinkedIn, Discord and GitHub releases
ad-square-brand | Square · brand | ads | 1080x1080 | 2 | 7.4 7.4 7.4 7.4 | - | - | Cold audiences on X, LinkedIn and Reddit feeds: one hook, the Field visual, no proof yet
ad-square-proof | Square · proof | ads | 1080x1080 | 3 | 7.4 7.4 7.4 7.4 | - | - | Warm audiences and retargeting: the approval dock in our UI: the agent asks, you decide
ad-portrait | Portrait feed | ads | 1080x1350 | 2 | 5.9 7.4 5.9 7.4 | - | - | Mobile feeds on LinkedIn, Reddit and Meta, where 4:5 takes the most screen: the handoff picker
ad-story | Story | ads | 1080x1920 | 3 | 4.2 7.4 4.2 7.4 | 14 0 20 0 | - | Full-screen vertical placements with the always-on agents; platform UI covers the top and bottom bands
ad-landscape | Landscape | ads | 1200x628 | 2 | 11.5 6 11.5 6 | - | - | Google display, Reddit link ads and Meta link previews, with the desktop
ad-x-card | X website card | ads | 800x418 | 2 | 13.4 6.5 13.4 6.5 | 0 0 14 0 | - | Promoted posts with a website card; X overlays the domain bottom left
ad-linkedin | LinkedIn single image | ads | 1200x627 | 3 | 11.5 5 11.5 5 | - | - | Sponsored content for engineering managers running agents across a team: the sessions board
ad-dev-banner | Sponsor banner | ads | 600x200 | 1 | 10 6 10 6 | - | - | GitHub sponsor slots and developer newsletters (TLDR, Console, Changelog)
og-default | OG · default | editorial | 1200x630 | 2 | 11.4 6 11.4 6 | - | - | Home and any page without its own image
og-blog | OG · blog | editorial | 1200x630 | 2 | 11.4 6 11.4 6 | - | - | Every blog post; title from front matter
og-docs | OG · docs | editorial | 1200x630 | 1 | 11.4 6 11.4 6 | - | - | Every docs page; section and title from the page tree
og-changelog | OG · changelog | editorial | 1200x630 | 2 | 11.4 6 11.4 6 | - | - | Release notes; version, date and three highlights
blog-hero | Blog hero | editorial | 1600x840 | 2 | 9.5 5 9.5 5 | - | - | Our UI as the art band of a product post; essays use Atmosphere; the title stays in HTML
newsletter-header | Newsletter header | editorial | 1200x400 | 2 | 15 5 15 5 | - | - | Top of every email, shown at 600 px wide
slide-title | Slide · title | editorial | 1920x1080 | 2 | 8.9 5 8.9 5 | - | - | First slide of a talk
slide-section | Slide · section | editorial | 1920x1080 | 1 | 8.9 5 8.9 5 | - | - | Divider between parts of a talk
slide-content | Slide · content | editorial | 1920x1080 | 0 | 8.9 5 8.9 5 | - | - | One claim and up to three points
slide-code | Slide · code | editorial | 1920x1080 | 3 | 8.9 5 8.9 5 | - | - | A claim proved by the file that implements it
slide-diagram | Slide · diagram | editorial | 1920x1080 | 3 | 8.9 5 8.9 5 | - | - | Orchestration drawn with our UI and agent faces
code-card | UI card | editorial | 1600x900 | 3 | 14.2 8 14.2 8 | - | - | Product UI as image for X, LinkedIn and the blog
call-background | Call background | editorial | 1920x1080 | 2 | 7.1 4 7.1 4 | - | - | Zoom, Meet and Teams; the centre stays empty for the face
wallpaper | Wallpaper | editorial | 2560x1440 | 2 | 5.3 3 5.3 3 | - | - | Desktop wallpaper for the team and the community
business-card-front | Business card · front | offline | 1004x650 | 2 | 12.4 8 12.4 8 | - | 85x55+3 | Name, role and contact; ink side
business-card-back | Business card · back | offline | 1004x650 | 2 | 12.4 8 12.4 8 | - | 85x55+3 | Ink side with a Swarm crop
letterhead | Letterhead A4 | offline | 2480x3508 | 1 | 5.6 9.5 4.3 9.5 | - | 210x297+3 | Letters, proposals and agreements printed or as PDF
envelope-dl | Envelope DL | offline | 2598x1299 | 1 | 13.6 6.8 13.6 6.8 | - | 220x110+0 | Letters and invitations, with address window
badge-team | Event badge · Team | offline | 1240x1748 | 2 | 10.7 8 5.7 8 | 7.5 0 0 0 | 105x148+3 | Team badge at Compozy events, A6 on a 20 mm lanyard
badge-speaker | Event badge · Speaker | offline | 1240x1748 | 2 | 10.7 8 5.7 8 | 7.5 0 0 0 | 105x148+3 | Speaker badge at Compozy events, A6 on a 20 mm lanyard
badge-partner | Event badge · Partner | offline | 1240x1748 | 2 | 10.7 8 5.7 8 | 7.5 0 0 0 | 105x148+3 | Partner badge at Compozy events, A6 on a 20 mm lanyard
badge-attendee | Event badge · Attendee | offline | 1240x1748 | 2 | 10.7 8 5.7 8 | 7.5 0 0 0 | 105x148+3 | Attendee badge at Compozy events, A6 on a 20 mm lanyard
sticker-sheet | Sticker sheet A5 | offline | 1748x2480 | 2 | 4.2 6 4.2 6 | - | 148x210+2 | Die-cut stickers (symbol, lines, three agent faces) for events and orders
poster-a3 | Poster A3 | offline | 3508x4961 | 2 | 4.9 7 4.9 7 | - | 297x420+3 | Meetups, office walls, event announcements
rollup | Roll-up 850×2000 | offline | 1700x4000 | 2 | 3.8 9 3.8 9 | 0 0 7.5 0 | 850x2000+5 | Event entrance and booth, with the desktop UI large; rendered 2 px/mm
tee-chest | Tee · centre chest | offline | 2953x1417 | 1 | 8 10 8 10 | - | 250x120+0 | Screen print on black cotton tees and hoodies
email-signature | Email signature | offline | 600x160 | 0 | 13.5 4 13.5 8 | - | - | Team email footer, PNG@2x or HTML
desktop-wallpaper | Desktop wallpaper | product | 2880x1800 | 1 | 6.7 4.2 6.7 4.2 | 2.7 0 6 0 | - | macOS desktop, also used as the empty desktop behind the Compozy shell`;

const num = (s: string | undefined) => {
  const n = Number(s);
  if (s === undefined || Number.isNaN(n)) throw new Error(`kit table: bad number ${s}`);
  return n;
};

const insets = (s: string): Insets | undefined => {
  if (s === "-") return undefined;
  const [t, r, b, l] = s.split(" ");
  return { t: num(t), r: num(r), b: num(b), l: num(l) };
};

const parse = (row: string): KitSpec => {
  const cols = row.split(" | ");
  if (cols.length !== 9) throw new Error(`kit table: expected 9 columns in "${row}"`);
  const [id, name, channel, size, level, margin, unsafe, print, use] = cols as [
    string,
    string,
    string,
    string,
    string,
    string,
    string,
    string,
    string,
  ];
  const [w, h] = size.split("x");
  const p = /^([\d.]+)x([\d.]+)\+([\d.]+)$/.exec(print);
  return {
    id,
    name,
    channel: channel as KitChannel,
    w: num(w),
    h: num(h),
    level: num(level) as KitSpec["level"],
    margin: insets(margin) ?? { t: 0, r: 0, b: 0, l: 0 },
    unsafe: insets(unsafe),
    print: p ? { wMm: num(p[1]), hMm: num(p[2]), bleedMm: num(p[3]) } : undefined,
    use,
  };
};

export const kitSpecs: KitSpec[] = TABLE.trim().split("\n").map(parse);

export const kitSpecById = (id: string): KitSpec => {
  const t = kitSpecs.find(x => x.id === id);
  if (!t) throw new Error(`unknown kit template ${id}`);
  return t;
};
