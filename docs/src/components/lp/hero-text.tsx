"use client";

import { motion } from "motion/react";

export default function HeroText() {
  return (
    <>
      <motion.div
        initial={{ opacity: 0, y: 20, filter: "blur(12px)" }}
        animate={{ opacity: 1, y: 0, filter: "blur(0px)" }}
        transition={{ duration: 0.8, ease: "easeOut" }}
      >
        <h1 className="mt-8 text-balance text-6xl md:text-7xl lg:mt-16 xl:text-[5.25rem] font-medium tracking-tight">
          Next-level Agentic{" "}
          <span className="bg-gradient-to-r from-[#E2F534] via-[#C1E623] to-[#96CD09] bg-clip-text text-transparent italic pr-[0.1em]">
            Orchestration
          </span>{" "}
          Platform
        </h1>
      </motion.div>
      <motion.div
        initial={{ opacity: 0, y: 20, filter: "blur(12px)" }}
        animate={{ opacity: 1, y: 0, filter: "blur(0px)" }}
        transition={{ duration: 0.8, delay: 0.2, ease: "easeOut" }}
      >
        <p className="mx-auto mt-8 max-w-4xl text-balance text-xl text-foreground/60 leading-relaxed">
          Orchestrate multi-agent AI systems with ease. Compozy’s enterprise-grade platform uses
          declarative YAML to deliver scalable, reliable, and cost-efficient distributed workflows,
          simplifying complex fan-outs, debugging, and monitoring for production-ready automation.
        </p>
      </motion.div>
    </>
  );
}
