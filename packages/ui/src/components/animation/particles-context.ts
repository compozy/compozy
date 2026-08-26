import { getStrictContext } from "../../lib/context";

export interface ParticlesContextType {
  animate: boolean;
  isInView: boolean;
}

export const [ParticlesProvider, useParticles] =
  getStrictContext<ParticlesContextType>("ParticlesContext");
