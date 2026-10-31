import { getStrictContext } from "@/components/site/strict-context";

export interface ParticlesContextType {
  animate: boolean;
  isInView: boolean;
}

export const [ParticlesProvider, useParticles] =
  getStrictContext<ParticlesContextType>("ParticlesContext");
