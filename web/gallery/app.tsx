import { prototypeParams } from "#gallery/contract";
import { PrototypeBoundary, PrototypeError, ReportReady } from "#gallery/render/boundary";
import { PrototypeCanvas } from "#gallery/render/canvas";
import { PrototypeToolbar } from "#gallery/render/toolbar";
import { usePrototype } from "#gallery/render/use-prototype";
import { useApplyTheme, useTheme } from "#gallery/render/use-theme";
import { Gallery } from "#gallery/shell/gallery";
import { loaderFor } from "#gallery/shell/registry";
import { useSearch } from "#gallery/use-search";

/** `/<slug>` renders one prototype standalone: the canvas iframe and screenshot URL. */
export function StandalonePrototype({ slug }: { slug: string }) {
  const search = useSearch();
  const theme = useTheme();
  const loaded = usePrototype(slug);
  if (!loaded) return null;
  if (!loaded.module) return <PrototypeError message={loaded.error ?? ""} />;
  const { meta, default: Prototype } = loaded.module;
  const params = new URLSearchParams(search);
  const requested = params.get(prototypeParams.state);
  const state = meta.states.find(option => option.id === requested)?.id ?? meta.states[0].id;
  return (
    <>
      <PrototypeCanvas key={`${slug}:${state}`} theme={theme}>
        <PrototypeBoundary slug={slug}>
          <Prototype state={state} />
          <ReportReady slug={slug} state={state} />
        </PrototypeBoundary>
      </PrototypeCanvas>
      {params.get(prototypeParams.chrome) === "0" ? null : (
        <PrototypeToolbar meta={meta} slug={slug} state={state} theme={theme} />
      )}
    </>
  );
}

/** Any path that names a prototype renders it standalone; everything else is the gallery. */
export function GalleryApp() {
  const theme = useTheme();
  useApplyTheme(theme);
  const slug = window.location.pathname.replace(/^\/+|\/+$/g, "");
  return loaderFor(slug) ? <StandalonePrototype slug={slug} /> : <Gallery />;
}
