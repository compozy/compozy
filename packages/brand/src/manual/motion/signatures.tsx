// 09 · 03 Signatures: ten live moves built from product state, atmosphere and text.
// Every stage renders its final frame; each card plays its demo once in view and on replay.
import { cn } from "@compozy/ui/utils";
import type { ComponentType } from "react";

import { lab } from "../cls";
import { Section } from "../parts/section";
import { signatures } from "./data";
import { SigCard } from "./sig-card";
import { CardMoveStage, DockAnswerStage, FaceStateStage, HandoffStage } from "./stages-product";
import {
  ConvergeStage,
  CounterStage,
  LaneGrowStage,
  LogLineStage,
  StateChangeStage,
  SweepStage,
} from "./stages-system";

// Product moves lead, in the order of the v1.2.0 chapter.
const stages: [id: string, Stage: ComponentType][] = [
  ["face-state", FaceStateStage],
  ["card-move", CardMoveStage],
  ["handoff", HandoffStage],
  ["dock-answer", DockAnswerStage],
  ["converge", ConvergeStage],
  ["sweep", SweepStage],
  ["lane-grow", LaneGrowStage],
  ["state-change", StateChangeStage],
  ["log-line", LogLineStage],
  ["counter", CounterStage],
];

const sig = (id: string) => {
  const s = signatures.find(x => x.id === id);
  if (!s) throw new Error(`unknown signature ${id}`);
  return s;
};

export function Signatures() {
  return (
    <Section
      id="signatures"
      n="03"
      file="motion/signatures/"
      title={
        <>
          Ten moves, one <em>state</em>
        </>
      }
      status="proposal"
      intro={
        <p>
          Every piece of Compozy motion is one of these ten, alone or in sequence. They play once
          when they scroll into view; replay any of them. Product moves lead: a face changing state,
          a card crossing the board, a handoff to another harness, the dock answering.
        </p>
      }
    >
      <div className="grid gap-4 md:grid-cols-2">
        {stages.map(([id, Stage]) => (
          <SigCard key={id} sig={sig(id)}>
            <Stage />
          </SigCard>
        ))}
      </div>
      <p className={cn(lab, "mt-6")}>
        Sequence rule: sting = converge → light sweep. Demo = face changes state → card crosses the
        board → handoff to another harness → dock answers → counter roll. Handoff and card moves
        only between named sessions. Terminal type-on is retired as a signature; it survives only in
        docs and CLI videos.
      </p>
    </Section>
  );
}
