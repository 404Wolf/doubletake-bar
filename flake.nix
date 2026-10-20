{
  description = "Waybar status module + GTK dropdown for doubletake (AirPlay sender for Linux)";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs { inherit system; };
        gtkDeps = [
          pkgs.gtk3
          pkgs.glib
          pkgs.gtk-layer-shell
        ];
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "doubletake-bar";
          version = "0.1.0";
          src = ./.;
          vendorHash = "sha256-vvplv6itph5breC2H9M7jV7AbpaykmfFJY21K81DwGI=";

          # GTK3 on this nixpkgs commit needs this build tag or gotk3's gdk
          # package fails to compile (an upstream version-detection bug).
          tags = [ "gtk_3_24" ];

          nativeBuildInputs = [ pkgs.pkg-config ];
          buildInputs = gtkDeps;

          meta = {
            description = "Waybar module + GTK dropdown for doubletake (AirPlay sender for Linux)";
            homepage = "https://github.com/404wolf/doubletake-bar";
            license = pkgs.lib.licenses.mit;
            mainProgram = "doubletake-bar";
            platforms = pkgs.lib.platforms.linux;
          };
        };

        devShells.default = pkgs.mkShell {
          packages = [
            pkgs.go
            pkgs.pkg-config
            pkgs.gopls
          ] ++ gtkDeps;
        };

        formatter = pkgs.nixfmt-rfc-style;
      }
    );
}
