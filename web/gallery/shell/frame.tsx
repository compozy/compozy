/**
 * One prototype in an iframe on its standalone URL (`frameSrc`), scaled down when the requested
 * width is wider than the area. `use-frame.ts` keeps the location in sync with `src`; `onElement`
 * hands the iframe to hosts that verify its status messages (flow previews).
 */
import { useFrame, type FrameInfo } from "#gallery/shell/use-frame";

type FrameProps = {
  src: string;
  title: string;
  width: number;
  height: number;
  scale: number;
  onInfo?: (info: FrameInfo | null) => void;
  onElement?: (element: HTMLIFrameElement | null) => void;
  tabIndex?: number;
};

export function Frame({
  src,
  title,
  width,
  height,
  scale,
  onInfo,
  onElement,
  tabIndex,
}: FrameProps) {
  const { ref, initialSrc, onLoad } = useFrame(src, onInfo);
  return (
    <iframe
      className="block origin-top-left border-0 bg-canvas"
      data-testid="prototype-frame"
      onLoad={onLoad}
      ref={element => {
        ref.current = element;
        onElement?.(element);
      }}
      src={initialSrc}
      style={{ width, height, transform: scale === 1 ? undefined : `scale(${scale})` }}
      tabIndex={tabIndex}
      title={title}
    />
  );
}
