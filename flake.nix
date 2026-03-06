{
  description = "Unix-style Matrix bots";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
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
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "messages";
          version = "0.1.0";
          src = ./.;
          vendorHash = "sha256-pskQHKrmMfNj6lc/Np5kkpUs0h3OgFSVHDsfpJ53MN0=";
          subPackages = [ "cmd/messages" ];

          meta = with pkgs.lib; {
            description = "Unix-style Matrix bot: listen | handler | send";
            homepage = "https://github.com/arjungandhi/messages";
            license = licenses.mit;
            mainProgram = "messages";
          };
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gopls
          ];
        };
      }
    );
}
