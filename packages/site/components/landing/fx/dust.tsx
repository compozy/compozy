"use client";

/** Drifting brand-coloured dust that parts around the pointer. Sleeps while it is off screen. */
import { useEffect, useRef } from "react";

const COLORS = ["31,135,252", "29,240,215", "181,250,246"];
const REACH = 160;

interface Mote {
  x: number;
  y: number;
  vx: number;
  vy: number;
  size: number;
  color: string;
  phase: number;
}

export function Dust({ count = 140, className }: { count?: number; className?: string }) {
  const ref = useRef<HTMLCanvasElement>(null);
  useEffect(() => {
    const canvas = ref.current;
    const context = canvas?.getContext("2d");
    if (!canvas || !context) return;
    let width = 0;
    let height = 0;
    let visible = true;
    let frame = 0;
    const pointer = { x: -9999, y: -9999 };
    const dpr = Math.min(window.devicePixelRatio, 2);
    const motes: Mote[] = [];
    const seed = () => {
      motes.length = 0;
      for (let index = 0; index < count; index += 1) {
        motes.push({
          x: Math.random() * width,
          y: Math.random() * height,
          vx: (Math.random() - 0.5) * 0.12,
          vy: -0.05 - Math.random() * 0.16,
          size: 0.5 + Math.random() ** 2 * 2,
          color: COLORS[index % COLORS.length],
          phase: Math.random() * Math.PI * 2,
        });
      }
    };
    const resize = () => {
      width = canvas.clientWidth;
      height = canvas.clientHeight;
      canvas.width = width * dpr;
      canvas.height = height * dpr;
      context.setTransform(dpr, 0, 0, dpr, 0, 0);
      seed();
    };
    const draw = (time: number) => {
      frame = requestAnimationFrame(draw);
      if (!visible) return;
      context.clearRect(0, 0, width, height);
      context.globalCompositeOperation = "lighter";
      for (const mote of motes) {
        const dx = mote.x - pointer.x;
        const dy = mote.y - pointer.y;
        const distance = Math.hypot(dx, dy);
        if (distance < REACH && distance > 0) {
          const push = (1 - distance / REACH) * 1.4;
          mote.x += (dx / distance) * push;
          mote.y += (dy / distance) * push;
        }
        mote.x = (mote.x + mote.vx + width) % width;
        mote.y = (mote.y + mote.vy + height) % height;
        const alpha = 0.25 + 0.55 * (0.5 + 0.5 * Math.sin(time / 900 + mote.phase));
        context.beginPath();
        context.fillStyle = `rgba(${mote.color},${alpha})`;
        context.arc(mote.x, mote.y, mote.size, 0, Math.PI * 2);
        context.fill();
      }
    };
    const move = (event: PointerEvent) => {
      const box = canvas.getBoundingClientRect();
      pointer.x = event.clientX - box.left;
      pointer.y = event.clientY - box.top;
    };
    const observer = new IntersectionObserver(([entry]) => {
      visible = entry.isIntersecting;
    });
    observer.observe(canvas);
    resize();
    window.addEventListener("resize", resize);
    window.addEventListener("pointermove", move);
    frame = requestAnimationFrame(draw);
    return () => {
      cancelAnimationFrame(frame);
      observer.disconnect();
      window.removeEventListener("resize", resize);
      window.removeEventListener("pointermove", move);
    };
  }, [count]);
  return <canvas ref={ref} aria-hidden className={className} />;
}
