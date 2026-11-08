import { kitSpecs, type KitChannel } from "../kit-specs";
import { Section } from "../parts/section";

const channels: KitChannel[] = ["social", "ads", "editorial", "offline", "product"];
const th = "px-4 py-3 font-medium";

export function Kit() {
  return (
    <Section
      id="kit"
      n="02"
      file="kit/index.json"
      title={
        <>
          The template <em>kit</em>
        </>
      }
      bg="ink2"
      status="proposal"
      intro={
        <p>
          {kitSpecs.length} templates, each a live component sized in container units. Edit text in
          place with query parameters, then export at native size.
        </p>
      }
    >
      <pre className="m-0 mb-8 overflow-x-auto rounded-card border border-(--b-line) bg-rail p-5 font-mono text-[13px] leading-[1.8]">
        <span className="text-muted-foreground">
          # all templates → $BRAND_EXPORT_DIR/kit/&lt;id&gt;.png (+ .pdf for print), digital assets
          → $BRAND_EXPORT_DIR/digital/ (default ~/Dev/compozy/_rebranding/exports)
        </span>
        {"\nbun run export:kit\n"}
        <span className="text-muted-foreground"># one template with new text</span>
        {'\nbun run export:kit -- --only x-post --set title="Loops now stop on their own."\n'}
        <span className="text-muted-foreground"># machine-readable list</span>
        {"\ncurl localhost:4620/kit/index.json"}
      </pre>
      <div className="grid min-w-0 gap-8">
        {channels.map(c => {
          const ts = kitSpecs.filter(t => t.channel === c);
          if (ts.length === 0) return null;
          return (
            <div key={c} className="min-w-0">
              <p className="m-0 mb-3 mono-label text-(--b-accent)">
                {c} · {ts.length}
              </p>
              <div className="overflow-x-auto rounded-card border border-(--b-line)">
                <table className="w-full min-w-[640px] border-collapse text-[14px]">
                  <thead>
                    <tr className="text-left mono-label text-[11px] tracking-[0.08em] text-(--b-muted)">
                      <th className={th}>Template</th>
                      <th className={th}>Size</th>
                      <th className={th}>Use</th>
                      <th className={th}>Level</th>
                      <th className={th} />
                    </tr>
                  </thead>
                  <tbody>
                    {ts.map(t => (
                      <tr key={t.id} className="border-t border-(--b-line)">
                        <td className="px-4 py-2.5 font-medium">{t.name}</td>
                        <td className="px-4 py-2.5 font-mono text-[12.5px] text-(--b-muted)">
                          {t.w}×{t.h}
                          {t.print && ` · ${t.print.wMm}×${t.print.hMm} mm`}
                        </td>
                        <td className="px-4 py-2.5 text-(--b-muted)">{t.use}</td>
                        <td className="px-4 py-2.5 font-mono text-[12.5px]">{t.level}</td>
                        <td className="px-4 py-2.5 text-right font-mono text-[12px] text-(--b-accent)">
                          <a href={`/kit/${t.id}`} className="no-underline hover:underline">
                            kit/{t.id} ↗
                          </a>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          );
        })}
      </div>
    </Section>
  );
}
