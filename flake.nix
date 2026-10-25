{
  description = "Native AirPlay tray menu for doubletake";
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };
  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
        # Apply to doubletake v0.5.0 for event-driven daemon snapshots.
        patches.default = ./patches/doubletake-watch.patch;
        packages.default = pkgs.buildGoModule {
          pname = "doubletake-bar";
          version = "0.2.0";
          src = ./.;
          vendorHash = "sha256-z6izC/+UUno4BxK4KpGP4f0BBqlgk/9zY2/USoenXOM=";
          env.CGO_ENABLED = 0;
          nativeBuildInputs = [ pkgs.makeWrapper ];
          postInstall = ''
            wrapProgram $out/bin/doubletake-bar --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.zenity ]}
          '';
          meta = {
            description = "Native AirPlay tray menu for doubletake";
            homepage = "https://github.com/404wolf/doubletake-bar";
            license = pkgs.lib.licenses.mit;
            mainProgram = "doubletake-bar";
            platforms = pkgs.lib.platforms.linux;
          };
        };
        devShells.default = pkgs.mkShell {
          packages = [
            pkgs.go
            pkgs.gopls
            pkgs.zenity
          ];
        };
        formatter = pkgs.nixfmt-rfc-style;
      }
    );
}
